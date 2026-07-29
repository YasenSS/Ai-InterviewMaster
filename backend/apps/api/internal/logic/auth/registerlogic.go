package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
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

func (l *RegisterLogic) Register(req *types.RegisterRequest) (*types.AuthResponse, error) {
	result, err := l.RegisterWithSession(req)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (l *RegisterLogic) RegisterWithSession(req *types.RegisterRequest) (*sessionResult, error) {
	email, emailOK := normalizeEmail(req.Email)
	displayName, nameOK := validateDisplayName(req.DisplayName)
	fields := make(map[string][]string)
	if !emailOK {
		fields["email"] = []string{"请输入合法邮箱，且长度不超过 254 个字符"}
	}
	if !validatePassword(req.Password) {
		fields["password"] = []string{"密码长度必须为 8–72 个字符"}
	}
	if !nameOK {
		fields["display_name"] = []string{"显示名称长度必须为 1–80 个字符"}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	tx, err := l.svcCtx.Database.Begin(l.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(l.ctx)

	var user types.UserResponse
	err = tx.QueryRow(l.ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id::text, email, display_name`,
		email,
		string(passwordHash),
		displayName,
	).Scan(&user.Id, &user.Email, &user.DisplayName)
	if err != nil {
		var pgErr *pgconn.PgError
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			(errors.As(err, &pgErr) && pgErr.Code == "23505") {
			return nil, apperror.New(
				"EMAIL_ALREADY_REGISTERED",
				"该邮箱已注册",
				http.StatusConflict,
				nil,
				nil,
			)
		}
		return nil, err
	}

	result, err := createSession(l.ctx, tx, l.svcCtx, user)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(l.ctx); err != nil {
		return nil, err
	}
	return result, nil
}
