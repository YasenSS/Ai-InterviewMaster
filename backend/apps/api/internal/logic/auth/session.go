package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"
	"github.com/jackc/pgx/v5"
)

const defaultRefreshTTL = 30 * 24 * time.Hour

type sessionResult struct {
	Response     *types.AuthResponse
	RefreshToken string
}

func normalizeEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 254 {
		return value, false
	}
	address, err := mail.ParseAddress(value)
	return value, err == nil && strings.EqualFold(address.Address, value)
}

func validatePassword(value string) bool {
	length := utf8.RuneCountInString(value)
	return length >= 8 && len([]byte(value)) <= 72
}

func validateDisplayName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	return value, length >= 1 && length <= 80
}

func createSession(
	ctx context.Context,
	tx pgx.Tx,
	svcCtx *svc.ServiceContext,
	user types.UserResponse,
) (*sessionResult, error) {
	refreshToken, err := randomToken()
	if err != nil {
		return nil, apperror.New(
			apperror.CodeInternal,
			"无法建立登录会话",
			500,
			nil,
			err,
		)
	}
	sessionID := uuid.NewString()
	ttl := time.Duration(svcCtx.Config.Auth.RefreshExpire) * time.Second
	if ttl <= 0 {
		ttl = defaultRefreshTTL
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		sessionID,
		user.Id,
		hashToken(refreshToken),
		time.Now().UTC().Add(ttl),
	)
	if err != nil {
		return nil, err
	}

	accessToken, err := platformauth.IssueForSession(
		user.Id,
		sessionID,
		svcCtx.Config.Auth.AccessSecret,
		time.Duration(svcCtx.Config.Auth.AccessExpire)*time.Second,
	)
	if err != nil {
		return nil, err
	}
	return &sessionResult{
		Response: &types.AuthResponse{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			ExpiresIn:   svcCtx.Config.Auth.AccessExpire,
			User:        user,
		},
		RefreshToken: refreshToken,
	}, nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func invalidSession() error {
	return apperror.Unauthorized("AUTH_SESSION_INVALID", "登录会话已失效，请重新登录")
}
