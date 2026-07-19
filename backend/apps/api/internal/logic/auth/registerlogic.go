// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterRequest) (resp *types.AuthResponse, err error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.DisplayName)
	if !strings.Contains(email, "@") || len(req.Password) < 8 || name == "" {
		return nil, fmt.Errorf("email、密码（至少 8 位）和显示名称均为必填项")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	var user types.UserResponse
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3) RETURNING id::text, email, display_name`, email, string(hash), name).
		Scan(&user.Id, &user.Email, &user.DisplayName)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	token, err := platformauth.Issue(user.Id, l.svcCtx.Config.Auth.AccessSecret, time.Duration(l.svcCtx.Config.Auth.AccessExpire)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	return &types.AuthResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: l.svcCtx.Config.Auth.AccessExpire, User: user}, nil
}
