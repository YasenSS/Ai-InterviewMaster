package auth

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	_, err := l.svcCtx.Database.Exec(l.ctx, `
		UPDATE refresh_sessions
		SET revoked_at = COALESCE(revoked_at, now()),
		    last_used_at = now()
		WHERE token_hash = $1`,
		hashToken(refreshToken),
	)
	return err
}
