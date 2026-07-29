package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
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

func (l *LoginLogic) Login(req *types.LoginRequest) (*types.AuthResponse, error) {
	result, err := l.LoginWithSession(req, "")
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (l *LoginLogic) LoginWithSession(req *types.LoginRequest, remoteIP string) (*sessionResult, error) {
	email, emailOK := normalizeEmail(req.Email)
	if !emailOK || req.Password == "" {
		return nil, apperror.Validation(map[string][]string{
			"email":    {"请输入合法邮箱"},
			"password": {"请输入密码"},
		})
	}
	if err := l.enforceRateLimit(email, remoteIP); err != nil {
		return nil, err
	}

	var passwordHash string
	var user types.UserResponse
	err := l.svcCtx.Database.QueryRow(l.ctx, `
		SELECT id::text, email, display_name, password_hash
		FROM users
		WHERE email = $1`,
		email,
	).Scan(&user.Id, &user.Email, &user.DisplayName, &passwordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "邮箱或密码错误")
	}

	tx, err := l.svcCtx.Database.Begin(l.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(l.ctx)
	result, err := createSession(l.ctx, tx, l.svcCtx, user)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(l.ctx); err != nil {
		return nil, err
	}
	l.clearRateLimit(email, remoteIP)
	return result, nil
}

func (l *LoginLogic) enforceRateLimit(email, remoteIP string) error {
	keys := []string{"auth:login:account:" + rateKey(email)}
	if strings.TrimSpace(remoteIP) != "" {
		keys = append(keys, "auth:login:ip:"+rateKey(remoteIP))
	}
	for _, key := range keys {
		count, err := l.svcCtx.Redis.Incr(l.ctx, key).Result()
		if err != nil {
			continue
		}
		if count == 1 {
			_ = l.svcCtx.Redis.Expire(l.ctx, key, 15*time.Minute).Err()
		}
		if count > 10 {
			return apperror.New(
				"AUTH_RATE_LIMITED",
				"登录尝试过于频繁，请稍后重试",
				http.StatusTooManyRequests,
				nil,
				nil,
			)
		}
	}
	return nil
}

func (l *LoginLogic) clearRateLimit(email, remoteIP string) {
	keys := []string{"auth:login:account:" + rateKey(email)}
	if strings.TrimSpace(remoteIP) != "" {
		keys = append(keys, "auth:login:ip:"+rateKey(remoteIP))
	}
	_ = l.svcCtx.Redis.Del(l.ctx, keys...).Err()
}

func rateKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
