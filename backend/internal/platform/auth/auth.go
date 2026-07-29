package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const userIDClaim = "user_id"
const sessionIDClaim = "session_id"

func Issue(userID, secret string, ttl time.Duration) (string, error) {
	return IssueForSession(userID, "", secret, ttl)
}

func IssueForSession(userID, sessionID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		userIDClaim: userID,
		"iat":       now.Unix(),
		"exp":       now.Add(ttl).Unix(),
	}
	if strings.TrimSpace(sessionID) != "" {
		claims[sessionIDClaim] = sessionID
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func UserID(ctx context.Context) (string, error) {
	value, ok := ctx.Value(userIDClaim).(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("authenticated user id is missing")
	}
	return value, nil
}

func SessionID(ctx context.Context) string {
	value, _ := ctx.Value(sessionIDClaim).(string)
	return strings.TrimSpace(value)
}

// UserIDFromToken verifies an access token before using its subject in audit
// metadata. Invalid or expired tokens deliberately return an empty value.
func UserIDFromToken(rawToken, secret string) string {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	value, _ := claims[userIDClaim].(string)
	return strings.TrimSpace(value)
}
