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

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (resp *types.AuthResponse, err error) {
	var hash string
	var user types.UserResponse
	err = l.svcCtx.Database.QueryRow(l.ctx, `SELECT id::text, email, display_name, password_hash FROM users WHERE email = $1`, strings.ToLower(strings.TrimSpace(req.Email))).
		Scan(&user.Id, &user.Email, &user.DisplayName, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		return nil, fmt.Errorf("邮箱或密码错误")
	}
	token, err := platformauth.Issue(user.Id, l.svcCtx.Config.Auth.AccessSecret, time.Duration(l.svcCtx.Config.Auth.AccessExpire)*time.Second)
	if err != nil { return nil, fmt.Errorf("issue access token: %w", err) }
	return &types.AuthResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: l.svcCtx.Config.Auth.AccessExpire, User: user}, nil
}
