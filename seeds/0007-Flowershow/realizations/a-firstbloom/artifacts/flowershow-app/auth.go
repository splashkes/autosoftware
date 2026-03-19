package main

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	cognitoidentityprovider "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/aws/smithy-go"
	"github.com/golang-jwt/jwt/v5"
)

const (
	authSessionCookieName = "as_flowershow_session"
	authPendingCookieName = "as_flowershow_auth_pending"
	authSessionDuration   = 33 * 24 * time.Hour
	emailOTPResendDelay   = 20 * time.Second

	pendingAuthFlowEmailOTP       = "email_otp"
	pendingAuthFlowForgotPassword = "forgot_password"
)

type authProvider interface {
	Enabled() bool
	PasswordLogin(ctx context.Context, email, password string) (*UserIdentity, error)
	StartEmailOTP(ctx context.Context, email string) (*authStartResult, error)
	VerifyEmailOTP(ctx context.Context, email, session, code string) (*UserIdentity, error)
	StartForgotPassword(ctx context.Context, email string) error
	ConfirmForgotPassword(ctx context.Context, email, code, newPassword string) error
}

type UserIdentity struct {
	SubjectID  string `json:"subject_id,omitempty"`
	CognitoSub string `json:"cognito_sub"`
	Email      string `json:"email,omitempty"`
	Name       string `json:"name,omitempty"`
}

type pendingAuthState struct {
	Flow              string `json:"flow"`
	Email             string `json:"email"`
	Session           string `json:"session,omitempty"`
	ExpiresAt         int64  `json:"expires_at"`
	ResendAvailableAt int64  `json:"resend_available_at,omitempty"`
}

type adminLoginData struct {
	Title           string
	CurrentPath     string
	Error           string
	Info            string
	CognitoEnabled  bool
	CurrentEmail    string
	PasswordStep    bool
	PendingEmail    string
	PendingEmailOTP bool
	PendingReset    bool
	PendingResendIn int
	BackHref        string
}

type accountNotice struct {
	Message string
	Kind    string
}

type authStartResult struct {
	User    *UserIdentity
	Pending *pendingAuthState
}

type authChallengeResult struct {
	AuthenticationResult *cognitotypes.AuthenticationResultType
	AvailableChallenges  []cognitotypes.ChallengeNameType
	ChallengeName        cognitotypes.ChallengeNameType
	Session              *string
}

type authDisplayError struct {
	Message string
}

func (e *authDisplayError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type cognitoIdentityAPI interface {
	InitiateAuth(context.Context, *cognitoidentityprovider.InitiateAuthInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.InitiateAuthOutput, error)
	RespondToAuthChallenge(context.Context, *cognitoidentityprovider.RespondToAuthChallengeInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.RespondToAuthChallengeOutput, error)
	ForgotPassword(context.Context, *cognitoidentityprovider.ForgotPasswordInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.ForgotPasswordOutput, error)
	ConfirmForgotPassword(context.Context, *cognitoidentityprovider.ConfirmForgotPasswordInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.ConfirmForgotPasswordOutput, error)
}

type cognitoAuth struct {
	userPoolID   string
	clientID     string
	clientSecret string
	region       string
	httpClient   *http.Client
	jwks         *jwksCache
	client       cognitoIdentityAPI
}

type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

type cognitoClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Sub   string `json:"sub"`
	jwt.RegisteredClaims
}

type cognitoJWKS struct {
	Keys []cognitoJWK `json:"keys"`
}

type cognitoJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func newAuthProviderFromEnv() (authProvider, error) {
	userPoolID := strings.TrimSpace(os.Getenv("AS_COGNITO_USER_POOL_ID"))
	clientID := strings.TrimSpace(os.Getenv("AS_COGNITO_CLIENT_ID"))
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if userPoolID == "" || clientID == "" || region == "" {
		return nil, nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config for cognito: %w", err)
	}

	return &cognitoAuth{
		userPoolID:   userPoolID,
		clientID:     clientID,
		clientSecret: strings.TrimSpace(os.Getenv("AS_COGNITO_CLIENT_SECRET")),
		region:       region,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		jwks: &jwksCache{
			keys: make(map[string]*rsa.PublicKey),
		},
		client: cognitoidentityprovider.NewFromConfig(cfg),
	}, nil
}

func (a *cognitoAuth) Enabled() bool { return a != nil }

func (a *cognitoAuth) RuntimeProvider() runtimeAuthProviderDescriptor {
	if a == nil {
		return runtimeAuthProviderDescriptor{}
	}
	issuer := ""
	if a.region != "" && a.userPoolID != "" {
		issuer = fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", a.region, a.userPoolID)
	}
	id := strings.TrimSpace(os.Getenv("AS_RUNTIME_AUTH_PROVIDER_ID"))
	if id == "" && issuer != "" && a.clientID != "" {
		id = "cognito:" + issuer + ":" + a.clientID
	}
	return runtimeAuthProviderDescriptor{
		ID:       id,
		Kind:     "cognito",
		Issuer:   issuer,
		ClientID: a.clientID,
	}
}

func (a *cognitoAuth) PasswordLogin(ctx context.Context, email, password string) (*UserIdentity, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return nil, &authDisplayError{Message: "Enter both email and password."}
	}

	resp, err := a.startUserAuth(ctx, email, cognitotypes.ChallengeNameTypePassword)
	if err != nil {
		return nil, a.translateAuthError(err, "The email or password was not accepted.")
	}

	for range 3 {
		if resp.AuthenticationResult != nil {
			return a.userFromAuthenticationResult(ctx, resp.AuthenticationResult)
		}
		switch resp.ChallengeName {
		case cognitotypes.ChallengeNameTypeSelectChallenge:
			resp, err = a.respondToAuthChallenge(ctx, email, valueOrEmpty(resp.Session), cognitotypes.ChallengeNameTypeSelectChallenge, map[string]string{
				"ANSWER":   string(cognitotypes.ChallengeNameTypePassword),
				"PASSWORD": password,
			})
		case cognitotypes.ChallengeNameTypePassword:
			resp, err = a.respondToAuthChallenge(ctx, email, valueOrEmpty(resp.Session), cognitotypes.ChallengeNameTypePassword, map[string]string{
				"PASSWORD": password,
			})
		case cognitotypes.ChallengeNameTypeNewPasswordRequired:
			return nil, &authDisplayError{Message: "This account requires a new password. Use the password reset flow below."}
		default:
			return nil, a.unsupportedChallengeError(resp.ChallengeName, resp.AvailableChallenges)
		}
		if err != nil {
			return nil, a.translateAuthError(err, "The email or password was not accepted.")
		}
	}

	return nil, &authDisplayError{Message: "The sign-in flow did not complete. Try again."}
}

func (a *cognitoAuth) StartEmailOTP(ctx context.Context, email string) (*authStartResult, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, &authDisplayError{Message: "Enter your email address to receive a code."}
	}

	resp, err := a.startUserAuth(ctx, email, cognitotypes.ChallengeNameTypeEmailOtp)
	if err != nil {
		return nil, a.translateAuthError(err, "Unable to start email-code sign-in.")
	}

	for range 3 {
		if resp.AuthenticationResult != nil {
			user, err := a.userFromAuthenticationResult(ctx, resp.AuthenticationResult)
			if err != nil {
				return nil, err
			}
			return &authStartResult{User: user}, nil
		}
		switch resp.ChallengeName {
		case cognitotypes.ChallengeNameTypeSelectChallenge:
			resp, err = a.respondToAuthChallenge(ctx, email, valueOrEmpty(resp.Session), cognitotypes.ChallengeNameTypeSelectChallenge, map[string]string{
				"ANSWER": string(cognitotypes.ChallengeNameTypeEmailOtp),
			})
			if err != nil {
				return nil, a.translateAuthError(err, "Unable to send an email sign-in code.")
			}
		case cognitotypes.ChallengeNameTypeEmailOtp:
			session := valueOrEmpty(resp.Session)
			if session == "" {
				return nil, &authDisplayError{Message: "Cognito did not return a sign-in session for the email code."}
			}
			return &authStartResult{
				Pending: &pendingAuthState{
					Flow:      pendingAuthFlowEmailOTP,
					Email:     email,
					Session:   session,
					ExpiresAt: time.Now().UTC().Add(15 * time.Minute).Unix(),
				},
			}, nil
		default:
			return nil, a.unsupportedChallengeError(resp.ChallengeName, resp.AvailableChallenges)
		}
	}

	return nil, &authDisplayError{Message: "The email sign-in flow did not complete. Try again."}
}

func (a *cognitoAuth) VerifyEmailOTP(ctx context.Context, email, session, code string) (*UserIdentity, error) {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
	if email == "" || session == "" {
		return nil, &authDisplayError{Message: "Request a fresh email code before verifying."}
	}
	if code == "" {
		return nil, &authDisplayError{Message: "Enter the email code to continue."}
	}

	resp, err := a.respondToAuthChallenge(ctx, email, session, cognitotypes.ChallengeNameTypeEmailOtp, map[string]string{
		"EMAIL_OTP_CODE": code,
	})
	if err != nil {
		return nil, a.translateAuthError(err, "The email code was not accepted.")
	}
	if resp.AuthenticationResult == nil {
		return nil, &authDisplayError{Message: "The email code did not complete sign-in. Request a new code and try again."}
	}
	return a.userFromAuthenticationResult(ctx, resp.AuthenticationResult)
}

func (a *cognitoAuth) StartForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return &authDisplayError{Message: "Enter your email address to reset the password."}
	}

	input := &cognitoidentityprovider.ForgotPasswordInput{
		ClientId: stringPtr(a.clientID),
		Username: stringPtr(email),
	}
	if secretHash := a.secretHash(email); secretHash != "" {
		input.SecretHash = stringPtr(secretHash)
	}
	if _, err := a.client.ForgotPassword(ctx, input); err != nil {
		return a.translateAuthError(err, "Unable to start password reset for that account.")
	}
	return nil
}

func (a *cognitoAuth) ConfirmForgotPassword(ctx context.Context, email, code, newPassword string) error {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
	newPassword = strings.TrimSpace(newPassword)
	if email == "" {
		return &authDisplayError{Message: "Enter the email address for the password reset."}
	}
	if code == "" || newPassword == "" {
		return &authDisplayError{Message: "Enter the reset code and a new password."}
	}

	input := &cognitoidentityprovider.ConfirmForgotPasswordInput{
		ClientId:         stringPtr(a.clientID),
		Username:         stringPtr(email),
		ConfirmationCode: stringPtr(code),
		Password:         stringPtr(newPassword),
	}
	if secretHash := a.secretHash(email); secretHash != "" {
		input.SecretHash = stringPtr(secretHash)
	}
	if _, err := a.client.ConfirmForgotPassword(ctx, input); err != nil {
		return a.translateAuthError(err, "The reset code or password was not accepted.")
	}
	return nil
}

func (a *cognitoAuth) startUserAuth(ctx context.Context, email string, preferredChallenge cognitotypes.ChallengeNameType) (*authChallengeResult, error) {
	authParameters := map[string]string{
		"USERNAME":            email,
		"PREFERRED_CHALLENGE": string(preferredChallenge),
	}
	if secretHash := a.secretHash(email); secretHash != "" {
		authParameters["SECRET_HASH"] = secretHash
	}
	resp, err := a.client.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow:       cognitotypes.AuthFlowTypeUserAuth,
		ClientId:       stringPtr(a.clientID),
		AuthParameters: authParameters,
	})
	if err != nil {
		return nil, err
	}
	return &authChallengeResult{
		AuthenticationResult: resp.AuthenticationResult,
		ChallengeName:        resp.ChallengeName,
		Session:              resp.Session,
	}, nil
}

func (a *cognitoAuth) respondToAuthChallenge(ctx context.Context, email, session string, challengeName cognitotypes.ChallengeNameType, extra map[string]string) (*authChallengeResult, error) {
	challengeResponses := map[string]string{
		"USERNAME": email,
	}
	if secretHash := a.secretHash(email); secretHash != "" {
		challengeResponses["SECRET_HASH"] = secretHash
	}
	for key, value := range extra {
		challengeResponses[key] = value
	}

	input := &cognitoidentityprovider.RespondToAuthChallengeInput{
		ChallengeName:      challengeName,
		ClientId:           stringPtr(a.clientID),
		ChallengeResponses: challengeResponses,
	}
	if session != "" {
		input.Session = stringPtr(session)
	}
	resp, err := a.client.RespondToAuthChallenge(ctx, input)
	if err != nil {
		return nil, err
	}
	return &authChallengeResult{
		AuthenticationResult: resp.AuthenticationResult,
		ChallengeName:        resp.ChallengeName,
		Session:              resp.Session,
	}, nil
}

func (a *cognitoAuth) secretHash(username string) string {
	if a.clientSecret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(a.clientSecret))
	mac.Write([]byte(username + a.clientID))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (a *cognitoAuth) userFromAuthenticationResult(ctx context.Context, result *cognitotypes.AuthenticationResultType) (*UserIdentity, error) {
	if result == nil || result.IdToken == nil || strings.TrimSpace(*result.IdToken) == "" {
		return nil, &authDisplayError{Message: "Cognito returned no identity token for this sign-in."}
	}
	claims, err := a.validateIDToken(ctx, strings.TrimSpace(*result.IdToken))
	if err != nil {
		return nil, err
	}
	return &UserIdentity{
		CognitoSub: claims.Sub,
		Email:      claims.Email,
		Name:       claims.Name,
	}, nil
}

func (a *cognitoAuth) unsupportedChallengeError(challenge cognitotypes.ChallengeNameType, available []cognitotypes.ChallengeNameType) error {
	if len(available) == 0 {
		return &authDisplayError{Message: fmt.Sprintf("Cognito returned an unsupported challenge: %s.", challenge)}
	}
	options := make([]string, 0, len(available))
	for _, candidate := range available {
		options = append(options, string(candidate))
	}
	return &authDisplayError{Message: fmt.Sprintf("This sign-in method is not currently available. Cognito offered: %s.", strings.Join(options, ", "))}
}

func (a *cognitoAuth) translateAuthError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	var display *authDisplayError
	if errors.As(err, &display) {
		return display
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotAuthorizedException":
			return &authDisplayError{Message: fallback}
		case "UserNotFoundException":
			return &authDisplayError{Message: "No Cognito account matched that email address."}
		case "CodeMismatchException":
			return &authDisplayError{Message: "The emailed code was not valid. Request a new code or try again."}
		case "ExpiredCodeException":
			return &authDisplayError{Message: "That code has expired. Request a new one and try again."}
		case "LimitExceededException", "TooManyRequestsException":
			return &authDisplayError{Message: "Too many attempts were made just now. Wait a moment and try again."}
		case "PasswordResetRequiredException":
			return &authDisplayError{Message: "This account needs a password reset before it can sign in."}
		case "InvalidParameterException":
			return &authDisplayError{Message: "The current Cognito client is not configured for that sign-in method yet."}
		case "UserNotConfirmedException":
			return &authDisplayError{Message: "This Cognito account is not confirmed yet."}
		}
	}
	return &authDisplayError{Message: fallback}
}

func (a *cognitoAuth) validateIDToken(ctx context.Context, raw string) (*cognitoClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &cognitoClaims{}, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		keys, err := a.fetchJWKS(ctx)
		if err != nil {
			return nil, err
		}
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown cognito key id %q", kid)
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*cognitoClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid cognito id token")
	}
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", a.region, a.userPoolID)
	if claims.Issuer != issuer {
		return nil, fmt.Errorf("unexpected cognito issuer %q", claims.Issuer)
	}
	if !audienceContains(claims.Audience, a.clientID) {
		return nil, errors.New("unexpected cognito audience")
	}
	return claims, nil
}

func audienceContains(audience jwt.ClaimStrings, want string) bool {
	for _, candidate := range audience {
		if candidate == want {
			return true
		}
	}
	return false
}

func (a *cognitoAuth) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	a.jwks.mu.RLock()
	if time.Now().Before(a.jwks.expiresAt) && len(a.jwks.keys) > 0 {
		keys := make(map[string]*rsa.PublicKey, len(a.jwks.keys))
		for k, v := range a.jwks.keys {
			keys[k] = v
		}
		a.jwks.mu.RUnlock()
		return keys, nil
	}
	a.jwks.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", a.region, a.userPoolID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch cognito jwks: %s", resp.Status)
	}

	var payload cognitoJWKS
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, jwk := range payload.Keys {
		key, err := rsaPublicKeyFromJWK(jwk)
		if err != nil {
			return nil, err
		}
		keys[jwk.Kid] = key
	}

	a.jwks.mu.Lock()
	a.jwks.keys = keys
	a.jwks.expiresAt = time.Now().Add(15 * time.Minute)
	a.jwks.mu.Unlock()
	return keys, nil
}

func rsaPublicKeyFromJWK(jwk cognitoJWK) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nb)
	e := new(big.Int).SetBytes(eb)
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

func (a *app) authEnabled() bool {
	return a.auth != nil && a.auth.Enabled()
}

func (a *app) currentUser(r *http.Request) (*UserIdentity, bool) {
	cookie, err := r.Cookie(authSessionCookieName)
	if err != nil {
		return nil, false
	}
	if a.sessions == nil {
		return nil, false
	}
	user, ok, err := a.sessions.ResolveUserSession(r.Context(), cookie.Value)
	if err != nil || !ok {
		return nil, false
	}
	return user, true
}

func (a *app) currentRoles(r *http.Request) []string {
	user, ok := a.currentUser(r)
	if !ok {
		return nil
	}
	return a.rolesForUser(*user)
}

func (a *app) roleAssignmentsForUser(user UserIdentity) []*UserRole {
	if a.authority == nil || strings.TrimSpace(user.subjectLookupKey()) == "" {
		return nil
	}
	roles, err := a.authority.RoleAssignmentsForUser(context.Background(), user)
	if err != nil {
		return nil
	}
	return roles
}

func (a *app) rolesForUser(user UserIdentity) []string {
	roles := make(map[string]struct{})
	for _, role := range a.roleAssignmentsForUser(user) {
		roles[role.Role] = struct{}{}
	}
	if a.authority != nil && strings.TrimSpace(user.SubjectID) != "" {
		if a.userHasCapability(context.Background(), user, "admin.dashboard.read", authorityScope{Kind: flowershowAuthorityScopeSeed, ID: flowershowAuthorityScopeSeedID}) {
			roles["admin"] = struct{}{}
		}
	}
	out := make([]string, 0, len(roles))
	for role := range roles {
		out = append(out, role)
	}
	return out
}

func (a *app) hasRole(r *http.Request, role string) bool {
	user, ok := a.currentUser(r)
	if !ok {
		return false
	}
	if a.authority != nil && role == "admin" && a.userHasCapability(r.Context(), *user, "admin.dashboard.read", authorityScope{Kind: flowershowAuthorityScopeSeed, ID: flowershowAuthorityScopeSeedID}) {
		return true
	}
	for _, candidate := range a.currentRoles(r) {
		if candidate == role {
			return true
		}
	}
	return false
}

func (a *app) isAdmin(r *http.Request) bool {
	return a.hasRole(r, "admin")
}

func (u UserIdentity) subjectLookupKey() string {
	if strings.TrimSpace(u.SubjectID) != "" {
		return strings.TrimSpace(u.SubjectID)
	}
	return strings.TrimSpace(u.CognitoSub)
}

func cookieSecure(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func (a *app) setUserSession(w http.ResponseWriter, r *http.Request, user UserIdentity) error {
	if a.sessions == nil {
		return errors.New("session store unavailable")
	}
	sessionID, err := a.sessions.CreateUserSession(r.Context(), user, r)
	if err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(authSessionDuration)
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(authSessionDuration / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
	})
	return nil
}

func (a *app) clearUserSession(w http.ResponseWriter, r *http.Request) {
	if a.sessions != nil {
		if cookie, err := r.Cookie(authSessionCookieName); err == nil {
			_ = a.sessions.EndUserSession(r.Context(), cookie.Value)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
	})
}

func (a *app) currentPendingAuth(r *http.Request) (*pendingAuthState, bool) {
	cookie, err := r.Cookie(authPendingCookieName)
	if err != nil {
		return nil, false
	}
	if a.sessions == nil {
		return nil, false
	}
	pending, ok, err := a.sessions.GetPendingAuth(r.Context(), cookie.Value)
	if err != nil || !ok {
		return nil, false
	}
	return pending, true
}

func (a *app) setPendingAuth(w http.ResponseWriter, r *http.Request, pending pendingAuthState) error {
	if a.sessions == nil {
		return errors.New("session store unavailable")
	}
	pendingID, err := a.sessions.CreatePendingAuth(r.Context(), pending)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authPendingCookieName,
		Value:    pendingID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(time.Until(time.Unix(pending.ExpiresAt, 0)).Seconds()),
		Secure:   cookieSecure(r),
	})
	return nil
}

func (a *app) clearPendingAuth(w http.ResponseWriter, r *http.Request) {
	if a.sessions != nil {
		if cookie, err := r.Cookie(authPendingCookieName); err == nil {
			_ = a.sessions.DeletePendingAuth(r.Context(), cookie.Value)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authPendingCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
	})
}

func (a *app) loginPageDataForState(errMessage, infoMessage, currentEmail string, passwordStep bool, pending *pendingAuthState) adminLoginData {
	data := adminLoginData{
		Title:          "Admin Login",
		CurrentPath:    "/admin/login",
		Error:          errMessage,
		Info:           infoMessage,
		CognitoEnabled: a.authEnabled(),
		CurrentEmail:   currentEmail,
		PasswordStep:   passwordStep && strings.TrimSpace(currentEmail) != "",
	}
	if pending != nil {
		data.PendingEmail = pending.Email
		data.PendingEmailOTP = pending.Flow == pendingAuthFlowEmailOTP
		data.PendingReset = pending.Flow == pendingAuthFlowForgotPassword
		if pending.Flow == pendingAuthFlowEmailOTP && pending.ResendAvailableAt > 0 {
			remaining := int(time.Until(time.Unix(pending.ResendAvailableAt, 0).UTC()).Round(time.Second).Seconds())
			if remaining < 0 {
				remaining = 0
			}
			data.PendingResendIn = remaining
		}
		data.CurrentEmail = pending.Email
		data.PasswordStep = false
	}
	if strings.TrimSpace(data.CurrentEmail) != "" {
		data.BackHref = "/admin/login?email=" + url.QueryEscape(data.CurrentEmail)
	} else {
		data.BackHref = "/admin/login"
	}
	return data
}

func (a *app) loginPageData(r *http.Request, errMessage, infoMessage string) adminLoginData {
	currentEmail := strings.TrimSpace(r.URL.Query().Get("email"))
	if currentEmail == "" {
		currentEmail = strings.TrimSpace(r.URL.Query().Get("prefill_email"))
	}
	passwordStep := strings.TrimSpace(r.URL.Query().Get("mode")) == "password"
	var pending *pendingAuthState
	if state, ok := a.currentPendingAuth(r); ok {
		pending = state
	}
	return a.loginPageDataForState(errMessage, infoMessage, currentEmail, passwordStep, pending)
}

func loginNoticeMessage(code string) string {
	switch strings.TrimSpace(code) {
	case "email-code-sent":
		return "Check your email for the secure code, then enter it below."
	case "password-reset-code-sent":
		return "Check your email for the password reset code, then set a new password below."
	case "password-reset-complete":
		return "Password updated. Sign in with the new password or request an email code."
	case "site-login-only":
		return "Sign in directly on this page. Hosted Cognito redirects are no longer used here."
	default:
		return ""
	}
}

func accountNoticeMessage(code string) accountNotice {
	switch strings.TrimSpace(code) {
	case "admin_required":
		return accountNotice{
			Message: "You are signed in, but this account does not currently have admin access.",
			Kind:    "info",
		}
	case "agent_token_revoked":
		return accountNotice{
			Message: "Agent token revoked.",
			Kind:    "info",
		}
	default:
		return accountNotice{}
	}
}

func (a *app) postLoginPathForRequest(r *http.Request) string {
	if a.isAdmin(r) {
		return "/admin"
	}
	return "/account"
}

func (a *app) postLoginPathForUser(user UserIdentity) string {
	for _, role := range a.rolesForUser(user) {
		if role == "admin" {
			return "/admin"
		}
	}
	return "/account"
}

func (a *app) renderAdminLogin(w http.ResponseWriter, r *http.Request, errMessage, infoMessage string) {
	a.render(w, "login.html", a.loginPageData(r, errMessage, infoMessage))
}

func (a *app) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(r); ok {
		http.Redirect(w, r, a.postLoginPathForRequest(r), http.StatusSeeOther)
		return
	}
	a.renderAdminLogin(w, r, "", loginNoticeMessage(r.URL.Query().Get("notice")))
}

func (a *app) renderAdminLoginWithState(w http.ResponseWriter, errMessage, infoMessage, currentEmail string, passwordStep bool, pending *pendingAuthState) {
	a.render(w, "login.html", a.loginPageDataForState(errMessage, infoMessage, currentEmail, passwordStep, pending))
}

func (a *app) handleAdminLoginBack(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	a.clearPendingAuth(w, r)
	location := "/admin/login"
	if email != "" {
		location += "?prefill_email=" + url.QueryEscape(email)
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func (a *app) handleAdminPasswordLogin(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		a.renderAdminLogin(w, r, "Cognito password sign-in is not configured for this deployment.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	user, err := a.auth.PasswordLogin(r.Context(), email, r.FormValue("password"))
	if err != nil {
		a.renderAdminLoginWithState(w, err.Error(), "", email, true, nil)
		return
	}
	if err := a.setUserSession(w, r, *user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.clearPendingAuth(w, r)
	http.Redirect(w, r, a.postLoginPathForUser(*user), http.StatusSeeOther)
}

func (a *app) handleAdminEmailCodeStart(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		a.renderAdminLogin(w, r, "Cognito email-code sign-in is not configured for this deployment.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	if pending, ok := a.currentPendingAuth(r); ok && pending.Flow == pendingAuthFlowEmailOTP && strings.EqualFold(strings.TrimSpace(pending.Email), email) {
		remaining := int(time.Until(time.Unix(pending.ResendAvailableAt, 0).UTC()).Round(time.Second).Seconds())
		if remaining > 0 {
			a.renderAdminLoginWithState(w, "", fmt.Sprintf("You can request another code in %ds.", remaining), email, false, pending)
			return
		}
	}
	result, err := a.auth.StartEmailOTP(r.Context(), email)
	if err != nil {
		a.renderAdminLoginWithState(w, err.Error(), "", email, false, nil)
		return
	}
	if result != nil && result.User != nil {
		if err := a.setUserSession(w, r, *result.User); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.clearPendingAuth(w, r)
		http.Redirect(w, r, a.postLoginPathForUser(*result.User), http.StatusSeeOther)
		return
	}
	if result == nil || result.Pending == nil {
		a.renderAdminLogin(w, r, "Cognito did not return an email-code challenge.", "")
		return
	}
	result.Pending.ResendAvailableAt = time.Now().UTC().Add(emailOTPResendDelay).Unix()
	if err := a.setPendingAuth(w, r, *result.Pending); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/login?notice=email-code-sent", http.StatusSeeOther)
}

func (a *app) handleAdminEmailCodeVerify(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		a.renderAdminLogin(w, r, "Cognito email-code sign-in is not configured for this deployment.", "")
		return
	}
	pending, ok := a.currentPendingAuth(r)
	if !ok || pending.Flow != pendingAuthFlowEmailOTP {
		a.renderAdminLogin(w, r, "Request a fresh email code before trying to verify it.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	user, err := a.auth.VerifyEmailOTP(r.Context(), pending.Email, pending.Session, r.FormValue("code"))
	if err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	if err := a.setUserSession(w, r, *user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.clearPendingAuth(w, r)
	http.Redirect(w, r, a.postLoginPathForUser(*user), http.StatusSeeOther)
}

func (a *app) handleAdminForgotPasswordStart(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		a.renderAdminLogin(w, r, "Cognito password reset is not configured for this deployment.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	if err := a.auth.StartForgotPassword(r.Context(), email); err != nil {
		a.renderAdminLoginWithState(w, err.Error(), "", email, false, nil)
		return
	}
	if err := a.setPendingAuth(w, r, pendingAuthState{
		Flow:      pendingAuthFlowForgotPassword,
		Email:     email,
		ExpiresAt: time.Now().UTC().Add(20 * time.Minute).Unix(),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/login?notice=password-reset-code-sent", http.StatusSeeOther)
}

func (a *app) handleAdminForgotPasswordConfirm(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		a.renderAdminLogin(w, r, "Cognito password reset is not configured for this deployment.", "")
		return
	}
	pending, ok := a.currentPendingAuth(r)
	if !ok || pending.Flow != pendingAuthFlowForgotPassword {
		a.renderAdminLogin(w, r, "Start a password reset before submitting a reset code.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")
	if newPassword != confirmPassword {
		a.renderAdminLogin(w, r, "The new passwords did not match.", "")
		return
	}
	if err := a.auth.ConfirmForgotPassword(r.Context(), pending.Email, r.FormValue("code"), newPassword); err != nil {
		a.renderAdminLogin(w, r, err.Error(), "")
		return
	}
	a.clearPendingAuth(w, r)
	http.Redirect(w, r, "/admin/login?notice=password-reset-complete", http.StatusSeeOther)
}

func (a *app) handleCognitoLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login?notice=site-login-only", http.StatusSeeOther)
}

func (a *app) handleCognitoCallback(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login?notice=site-login-only", http.StatusSeeOther)
}

func (a *app) handleCognitoLogout(w http.ResponseWriter, r *http.Request) {
	a.clearPendingAuth(w, r)
	a.clearUserSession(w, r)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

type roleManagementData struct {
	Title       string
	CurrentPath string
	User        *UserIdentity
	Roles       []*UserRole
	Shows       []*Show
	Orgs        []*Organization
}

func (a *app) handleRoleManagement(w http.ResponseWriter, r *http.Request) {
	user, _ := a.currentUser(r)
	roles := []*UserRole{}
	if a.authority != nil {
		if items, err := a.authority.AllRoleAssignments(r.Context()); err == nil {
			roles = items
		}
	}
	a.render(w, "admin_roles.html", roleManagementData{
		Title:       "Role Management",
		CurrentPath: "/admin/roles",
		User:        user,
		Roles:       roles,
		Shows:       a.store.allShows(),
		Orgs:        a.store.allOrganizations(),
	})
}

func (a *app) handleRoleAssign(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if a.authority == nil {
		http.Error(w, "runtime authority unavailable", http.StatusServiceUnavailable)
		return
	}
	user, ok := a.currentUser(r)
	if !ok {
		http.Error(w, "signed-in user required", http.StatusUnauthorized)
		return
	}
	_, err := a.authority.AssignRole(r.Context(), UserRoleInput{
		CognitoSub:     r.FormValue("cognito_sub"),
		OrganizationID: r.FormValue("organization_id"),
		ShowID:         r.FormValue("show_id"),
		Role:           r.FormValue("role"),
	}, user.SubjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/roles", http.StatusSeeOther)
}

func stringPtr(s string) *string {
	return &s
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
