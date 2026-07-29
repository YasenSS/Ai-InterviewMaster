package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

const Header = "X-Request-ID"

type contextKey struct{}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,128}$`)

func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, Ensure(w, r))
	}
}

// Ensure makes request correlation available even for responses emitted by
// authentication middleware before the normal middleware chain is entered.
func Ensure(w http.ResponseWriter, r *http.Request) *http.Request {
	id := FromContext(r.Context())
	if id == "" {
		id = r.Header.Get(Header)
		if !validRequestID.MatchString(id) {
			id = New()
		}
	}
	w.Header().Set(Header, id)
	ctx := context.WithValue(r.Context(), contextKey{}, id)
	return r.WithContext(ctx)
}

func FromContext(ctx context.Context) string {
	value, _ := ctx.Value(contextKey{}).(string)
	return value
}

func New() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
