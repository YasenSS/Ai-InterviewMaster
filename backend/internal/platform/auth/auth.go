package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const userIDClaim = "user_id"

func Issue(userID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		userIDClaim: userID,
		"iat":        now.Unix(),
		"exp":        now.Add(ttl).Unix(),
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
