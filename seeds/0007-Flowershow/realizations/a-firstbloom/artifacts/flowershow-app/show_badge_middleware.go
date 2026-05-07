package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// showBadgeCookieName is the cookie that carries a redeemed share-link
// session. It is intentionally separate from authSessionCookieName because
// helper sessions grant no Cognito-side identity and do not satisfy admin
// auth flows.
const showBadgeCookieName = "as_show_badge"

// showBadgeContextKey is the request-context key for the resolved badge
// session. Handlers behind requireShowBadge can recover the session via
// showBadgeFromContext.
type showBadgeContextKeyType struct{}

var showBadgeContextKey = showBadgeContextKeyType{}

func showBadgeFromContext(ctx context.Context) (*ShowBadgeSession, bool) {
	if ctx == nil {
		return nil, false
	}
	session, ok := ctx.Value(showBadgeContextKey).(*ShowBadgeSession)
	if !ok || session == nil {
		return nil, false
	}
	return session, true
}

func (a *app) currentShowBadge(r *http.Request) (*ShowBadgeSession, bool) {
	if r == nil {
		return nil, false
	}
	cookie, err := r.Cookie(showBadgeCookieName)
	if err != nil {
		return nil, false
	}
	session, ok := a.store.findShowBadgeSessionByToken(cookie.Value)
	if !ok || session == nil {
		return nil, false
	}
	return session, true
}

func (a *app) setShowBadgeCookie(w http.ResponseWriter, r *http.Request, plaintext string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     showBadgeCookieName,
		Value:    plaintext,
		Path:     "/shows/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
	})
}

func (a *app) clearShowBadgeCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     showBadgeCookieName,
		Value:    "",
		Path:     "/shows/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
	})
}

// requireShowBadge gates a handler so only redeemed helpers may reach it.
// It validates the cookie, ensures the session belongs to the requested
// show (when {slug} is in the path), and pushes the session into the
// request context for the inner handler to consume. Anyone without a valid
// badge is sent to /shows/{slug}/help-redeem with a `next` parameter so
// they can return to the original target after redeeming.
func (a *app) requireShowBadge(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimSpace(r.PathValue("slug"))
		session, ok := a.currentShowBadge(r)
		if !ok {
			a.redirectToShowBadgeRedeem(w, r, slug)
			return
		}
		if slug != "" {
			show, found := a.store.showBySlug(slug)
			if !found || show == nil {
				http.NotFound(w, r)
				return
			}
			if session.ShowID != show.ID {
				a.redirectToShowBadgeRedeem(w, r, slug)
				return
			}
		}
		// Best-effort touch — failures are non-fatal so a flaky DB does
		// not lock helpers out of pages they already validated for.
		_ = a.store.touchShowBadgeSession(session.ID)
		ctx := context.WithValue(r.Context(), showBadgeContextKey, session)
		next(w, r.WithContext(ctx))
	}
}

func (a *app) redirectToShowBadgeRedeem(w http.ResponseWriter, r *http.Request, slug string) {
	target := "/shows"
	if slug != "" {
		target = "/shows/" + slug + "/help-redeem"
	} else {
		target = "/shows"
	}
	if r != nil && strings.TrimSpace(r.URL.RequestURI()) != "" {
		next := url.QueryEscape(r.URL.RequestURI())
		if strings.Contains(target, "?") {
			target += "&next=" + next
		} else {
			target += "?next=" + next
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
