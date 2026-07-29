// Current-user profile and password changes share the authenticated user lookup.
package workspace

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

func loadUser(ctx context.Context, svcCtx *svc.ServiceContext, userID string) (*types.UserResponse, error) {
	response := &types.UserResponse{}
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT id::text, email, display_name
		FROM users
		WHERE id = $1`,
		userID,
	).Scan(&response.Id, &response.Email, &response.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.Unauthorized("AUTH_SESSION_INVALID", "登录用户已不存在")
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

type MeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeLogic {
	return &MeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MeLogic) Me() (*types.UserResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return loadUser(l.ctx, l.svcCtx, userID)
}

type UpdateMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMeLogic {
	return &UpdateMeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateMeLogic) UpdateMe(req *types.UpdateMeRequest) (*types.UserResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	length := utf8.RuneCountInString(displayName)
	if length < 1 || length > 80 {
		return nil, apperror.Validation(map[string][]string{
			"display_name": {"显示名称长度必须为 1–80 个字符"},
		})
	}
	response := &types.UserResponse{}
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		UPDATE users
		SET display_name = $2, updated_at = now()
		WHERE id = $1
		RETURNING id::text, email, display_name`,
		userID,
		displayName,
	).Scan(&response.Id, &response.Email, &response.DisplayName)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type ChangePasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePasswordLogic {
	return &ChangePasswordLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ChangePasswordLogic) ChangePassword(req *types.ChangePasswordRequest) error {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return err
	}
	if !validPassword(req.NewPassword) {
		return apperror.Validation(map[string][]string{
			"new_password": {"新密码长度必须为 8–72 个字符"},
		})
	}
	tx, err := l.svcCtx.Database.Begin(l.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(l.ctx)
	var currentHash string
	err = tx.QueryRow(l.ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1
		FOR UPDATE`,
		userID,
	).Scan(&currentHash)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)) != nil {
		return apperror.New(
			"CURRENT_PASSWORD_INCORRECT",
			"当前密码不正确",
			http.StatusBadRequest,
			nil,
			nil,
		)
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.NewPassword)) == nil {
		return apperror.New(
			"PASSWORD_UNCHANGED",
			"新密码不能与当前密码相同",
			http.StatusBadRequest,
			nil,
			nil,
		)
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(l.ctx, `
		UPDATE users
		SET password_hash = $2, updated_at = now()
		WHERE id = $1`,
		userID,
		string(newHash),
	); err != nil {
		return err
	}
	sessionID := platformauth.SessionID(l.ctx)
	if _, err := tx.Exec(l.ctx, `
		UPDATE refresh_sessions
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE user_id = $1
		  AND ($2 = '' OR id::text <> $2)`,
		userID,
		sessionID,
	); err != nil {
		return err
	}
	return tx.Commit(l.ctx)
}

func validPassword(value string) bool {
	length := utf8.RuneCountInString(value)
	return length >= 8 && len([]byte(value)) <= 72
}
