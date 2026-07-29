package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshLogic) Refresh(refreshToken string) (*types.AuthResponse, string, error) {
	if refreshToken == "" {
		return nil, "", invalidSession()
	}
	tx, err := l.svcCtx.Database.Begin(l.ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(l.ctx)

	var oldSessionID string
	var expiresAt time.Time
	var revokedAt *time.Time
	var user types.UserResponse
	err = tx.QueryRow(l.ctx, `
		SELECT session.id::text,
		       session.expires_at,
		       session.revoked_at,
		       users.id::text,
		       users.email,
		       users.display_name
		FROM refresh_sessions AS session
		JOIN users ON users.id = session.user_id
		WHERE session.token_hash = $1
		FOR UPDATE`,
		hashToken(refreshToken),
	).Scan(
		&oldSessionID,
		&expiresAt,
		&revokedAt,
		&user.Id,
		&user.Email,
		&user.DisplayName,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, "", invalidSession()
		}
		return nil, "", err
	}
	if revokedAt != nil || !expiresAt.After(time.Now().UTC()) {
		return nil, "", invalidSession()
	}

	newRefreshToken, err := randomToken()
	if err != nil {
		return nil, "", err
	}
	newSessionID := uuid.NewString()
	ttl := time.Duration(l.svcCtx.Config.Auth.RefreshExpire) * time.Second
	if ttl <= 0 {
		ttl = defaultRefreshTTL
	}
	_, err = tx.Exec(l.ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		newSessionID,
		user.Id,
		hashToken(newRefreshToken),
		time.Now().UTC().Add(ttl),
	)
	if err != nil {
		return nil, "", err
	}
	_, err = tx.Exec(l.ctx, `
		UPDATE refresh_sessions
		SET revoked_at = now(),
		    last_used_at = now(),
		    replaced_by_session_id = $2
		WHERE id = $1`,
		oldSessionID,
		newSessionID,
	)
	if err != nil {
		return nil, "", err
	}
	accessToken, err := platformauth.IssueForSession(
		user.Id,
		newSessionID,
		l.svcCtx.Config.Auth.AccessSecret,
		time.Duration(l.svcCtx.Config.Auth.AccessExpire)*time.Second,
	)
	if err != nil {
		return nil, "", err
	}
	if err := tx.Commit(l.ctx); err != nil {
		return nil, "", err
	}
	return &types.AuthResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   l.svcCtx.Config.Auth.AccessExpire,
		User:        user,
	}, newRefreshToken, nil
}
