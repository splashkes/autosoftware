package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

type cspNonceKey struct{}

type SecurityHeadersOptions struct {
	ContentSecurityPolicy string
	ReferrerPolicy        string
	FrameOptions          string
	PermissionsPolicy     string
}

type SameOriginUnsafeMethodsOptions struct {
	AllowedOrigins      []string
	TrustForwardedProto bool
}

func DefaultMiddlewareStack(next http.Handler) http.Handler {
	return SecurityHeadersMiddleware(
		CorrelationMiddleware(
			SameOriginUnsafeMethodsMiddleware(next),
		),
	)
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return SecurityHeadersMiddlewareWithOptions(SecurityHeadersOptions{}, next)
}

func SecurityHeadersMiddlewareWithOptions(options SecurityHeadersOptions, next http.Handler) http.Handler {
	referrerPolicy := firstNonEmpty(strings.TrimSpace(options.ReferrerPolicy), "strict-origin-when-cross-origin")
	frameOptions := firstNonEmpty(strings.TrimSpace(options.FrameOptions), "DENY")
	permissionsPolicy := strings.TrimSpace(options.PermissionsPolicy)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := newCSPNonce()
		ctx := context.WithValue(r.Context(), cspNonceKey{}, nonce)

		w.Header().Set("Content-Security-Policy", defaultContentSecurityPolicy(nonce, options.ContentSecurityPolicy))
		w.Header().Set("Referrer-Policy", referrerPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", frameOptions)
		if permissionsPolicy != "" {
			w.Header().Set("Permissions-Policy", permissionsPolicy)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SameOriginUnsafeMethodsMiddleware(next http.Handler) http.Handler {
	return SameOriginUnsafeMethodsMiddlewareWithOptions(SameOriginUnsafeMethodsOptions{}, next)
}

func SameOriginUnsafeMethodsMiddlewareWithOptions(options SameOriginUnsafeMethodsOptions, next http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{}, len(options.AllowedOrigins))
	for _, origin := range options.AllowedOrigins {
		if normalized := normalizeOrigin(origin); normalized != "" {
			allowedOrigins[normalized] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isUnsafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
		switch fetchSite {
		case "", "same-origin", "none":
		default:
			http.Error(w, "cross-origin browser writes are not allowed", http.StatusForbidden)
			return
		}

		target := requestOrigin(r, options.TrustForwardedProto)

		if origin := normalizeOrigin(r.Header.Get("Origin")); origin != "" {
			if !originAllowed(origin, target, allowedOrigins) {
				http.Error(w, "origin mismatch", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if referer := strings.TrimSpace(r.Referer()); referer != "" {
			referrerOrigin := normalizeOrigin(referer)
			if referrerOrigin == "" || !originAllowed(referrerOrigin, target, allowedOrigins) {
				http.Error(w, "referrer mismatch", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func CSPNonceFromContext(ctx context.Context) string {
	nonce, _ := ctx.Value(cspNonceKey{}).(string)
	return nonce
}

func defaultContentSecurityPolicy(nonce string, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return strings.ReplaceAll(configured, "$NONCE", nonce)
	}

	return strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"connect-src 'self'",
		"font-src 'self' data:",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"img-src 'self' data:",
		"manifest-src 'self'",
		"object-src 'none'",
		"script-src 'self' 'nonce-" + nonce + "'",
		"style-src 'self' 'nonce-" + nonce + "'",
	}, "; ")
}

func newCSPNonce() string {
	var raw [18]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}

	return base64.RawStdEncoding.EncodeToString(raw[:])
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func requestOrigin(r *http.Request, trustForwardedProto bool) string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if trustForwardedProto {
		switch forwarded := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); forwarded {
		case "http", "https":
			scheme = forwarded
		}
	}

	return scheme + "://" + host
}

func originAllowed(origin, targetOrigin string, allowedOrigins map[string]struct{}) bool {
	if origin == "" {
		return false
	}
	if targetOrigin != "" && origin == targetOrigin {
		return true
	}
	_, ok := allowedOrigins[origin]
	return ok
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}
