package auth

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/config"
)

func refreshCookie(c config.Config, value string) *http.Cookie {
	maxAge := int(c.Auth.RefreshExpire)
	if maxAge <= 0 {
		maxAge = 30 * 24 * 60 * 60
	}
	name := strings.TrimSpace(c.Auth.RefreshCookieName)
	if name == "" {
		name = "interviewmaster_refresh"
	}
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/api/v1/auth",
		MaxAge:   maxAge,
		Expires:  time.Now().UTC().Add(time.Duration(maxAge) * time.Second),
		HttpOnly: true,
		Secure:   strings.EqualFold(c.Runtime.Environment, "production"),
		SameSite: http.SameSiteLaxMode,
	}
}

func clearRefreshCookie(c config.Config) *http.Cookie {
	cookie := refreshCookie(c, "")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0).UTC()
	return cookie
}

func readRefreshCookie(r *http.Request, c config.Config) string {
	name := strings.TrimSpace(c.Auth.RefreshCookieName)
	if name == "" {
		name = "interviewmaster_refresh"
	}
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
