package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
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

	"github.com/golang-jwt/jwt/v5"
)

const (
	authSessionCookieName = "as_flowershow_session"
	authStateCookieName   = "as_flowershow_auth_state"
)

type authProvider interface {
	Enabled() bool
	LoginURL(state string) string
	LogoutURL() string
	ExchangeCode(ctx context.Context, code string) (*UserIdentity, error)
}

type UserIdentity struct {
	CognitoSub string `json:"cognito_sub"`
	Email      string `json:"email,omitempty"`
	Name       string `json:"name,omitempty"`
}

type authSession struct {
	User      UserIdentity `json:"user"`
	ExpiresAt int64        `json:"expires_at"`
}

type cognitoAuth struct {
	userPoolID string
	clientID   string
	domain     string
	region     string
	redirect   string
	logout     string
	httpClient *http.Client
	jwks       *jwksCache
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

type cognitoTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
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
	domain := strings.TrimSpace(os.Getenv("AS_COGNITO_DOMAIN"))
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	redirectURL := strings.TrimSpace(os.Getenv("AS_COGNITO_REDIRECT_URL"))
	if userPoolID == "" || clientID == "" || domain == "" || region == "" || redirectURL == "" {
		return nil, nil
	}
	if !strings.HasPrefix(domain, "https://") {
		domain = "https://" + strings.TrimPrefix(domain, "http://")
	}
	return &cognitoAuth{
		userPoolID: userPoolID,
		clientID:   clientID,
		domain:     strings.TrimRight(domain, "/"),
		region:     region,
		redirect:   redirectURL,
		logout:     strings.TrimSpace(os.Getenv("AS_COGNITO_LOGOUT_URL")),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		jwks: &jwksCache{
			keys: make(map[string]*rsa.PublicKey),
		},
	}, nil
}

func (a *cognitoAuth) Enabled() bool { return a != nil }

func (a *cognitoAuth) LoginURL(state string) string {
	q := url.Values{}
	q.Set("client_id", a.clientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("redirect_uri", a.redirect)
	q.Set("state", state)
	return a.domain + "/login?" + q.Encode()
}

func (a *cognitoAuth) LogoutURL() string {
	if a.logout == "" {
		return ""
	}
	q := url.Values{}
	q.Set("client_id", a.clientID)
	q.Set("logout_uri", a.logout)
	return a.domain + "/logout?" + q.Encode()
}

func (a *cognitoAuth) ExchangeCode(ctx context.Context, code string) (*UserIdentity, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientID)
	form.Set("code", code)
	form.Set("redirect_uri", a.redirect)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.domain+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cognito token exchange failed: %s", resp.Status)
	}

	var tokens cognitoTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	if tokens.IDToken == "" {
		return nil, errors.New("cognito token exchange returned no id_token")
	}

	claims, err := a.validateIDToken(ctx, tokens.IDToken)
	if err != nil {
		return nil, err
	}
	return &UserIdentity{
		CognitoSub: claims.Sub,
		Email:      claims.Email,
		Name:       claims.Name,
	}, nil
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

func newSessionSecret() []byte {
	secret := strings.TrimSpace(os.Getenv("AS_SESSION_SECRET"))
	if secret != "" {
		return []byte(secret)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err == nil {
		return buf
	}
	return []byte("flowershow-dev-secret")
}

func randomToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func (a *app) authEnabled() bool {
	return a.auth != nil && a.auth.Enabled()
}

func (a *app) currentUser(r *http.Request) (*UserIdentity, bool) {
	cookie, err := r.Cookie(authSessionCookieName)
	if err != nil {
		return nil, false
	}
	session, ok := a.parseSessionCookie(cookie.Value)
	if !ok || time.Now().UTC().Unix() >= session.ExpiresAt {
		return nil, false
	}
	return &session.User, true
}

func (a *app) currentRoles(r *http.Request) []string {
	user, ok := a.currentUser(r)
	if !ok {
		return nil
	}
	roles := make(map[string]struct{})
	for _, role := range a.store.rolesBySubject(user.CognitoSub) {
		roles[role.Role] = struct{}{}
	}
	if user.Email != "" && a.bootstrapAdmins[strings.ToLower(user.Email)] {
		roles["admin"] = struct{}{}
	}
	out := make([]string, 0, len(roles))
	for role := range roles {
		out = append(out, role)
	}
	return out
}

func (a *app) hasRole(r *http.Request, role string) bool {
	for _, candidate := range a.currentRoles(r) {
		if candidate == role {
			return true
		}
	}
	return false
}

func (a *app) isAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookieName)
	if err == nil && cookie.Value == "ok" {
		return true
	}
	return a.hasRole(r, "admin")
}

func (a *app) encodeSessionCookie(session authSession) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, a.sessionSecret)
	mac.Write(payload)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(sig), nil
}

func (a *app) parseSessionCookie(raw string) (*authSession, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	mac := hmac.New(sha256.New, a.sessionSecret)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, false
	}
	var session authSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, false
	}
	return &session, true
}

func (a *app) setUserSession(w http.ResponseWriter, user UserIdentity) error {
	value, err := a.encodeSessionCookie(authSession{
		User:      user,
		ExpiresAt: time.Now().UTC().Add(8 * time.Hour).Unix(),
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *app) clearUserSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func bootstrapAdminMap() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("AS_COGNITO_ADMIN_EMAILS"))
	if raw == "" {
		return map[string]bool{}
	}
	out := make(map[string]bool)
	for _, email := range strings.Split(raw, ",") {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" {
			out[email] = true
		}
	}
	return out
}

func (a *app) handleCognitoLogin(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		http.NotFound(w, r)
		return
	}
	state := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     authStateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})
	http.Redirect(w, r, a.auth.LoginURL(state), http.StatusSeeOther)
}

func (a *app) handleCognitoCallback(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled() {
		http.NotFound(w, r)
		return
	}
	stateCookie, err := r.Cookie(authStateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid auth state", http.StatusBadRequest)
		return
	}
	user, err := a.auth.ExchangeCode(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := a.setUserSession(w, *user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *app) handleCognitoLogout(w http.ResponseWriter, r *http.Request) {
	a.clearUserSession(w)
	if a.authEnabled() {
		if url := a.auth.LogoutURL(); url != "" {
			http.Redirect(w, r, url, http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

type roleManagementData struct {
	Title string
	User  *UserIdentity
	Roles []*UserRole
	Shows []*Show
	Orgs  []*Organization
}

func (a *app) handleRoleManagement(w http.ResponseWriter, r *http.Request) {
	user, _ := a.currentUser(r)
	a.render(w, "admin_roles.html", roleManagementData{
		Title: "Role Management",
		User:  user,
		Roles: a.store.allUserRoles(),
		Shows: a.store.allShows(),
		Orgs:  a.store.allOrganizations(),
	})
}

func (a *app) handleRoleAssign(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := a.store.assignUserRole(UserRoleInput{
		CognitoSub:     r.FormValue("cognito_sub"),
		OrganizationID: r.FormValue("organization_id"),
		ShowID:         r.FormValue("show_id"),
		Role:           r.FormValue("role"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/roles", http.StatusSeeOther)
}
