package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/jackc/pgx/v5"
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
	inviteCode, inviteOK := normalizeInviteCode(req.InviteCode)
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
	if !inviteOK {
		fields["invite_code"] = []string{"请输入有效的邀请码"}
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
	if err := consumeInviteCode(l.ctx, tx, inviteCode, email); err != nil {
		return nil, err
	}

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

func normalizeInviteCode(value string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(value))
	return code, code != "" && len(code) <= 128
}

func consumeInviteCode(ctx context.Context, tx pgx.Tx, code, email string) error {
	sum := sha256.Sum256([]byte(code))
	var id string
	err := tx.QueryRow(ctx, `
		UPDATE invite_codes
		SET used_count = used_count + 1, last_used_at = now()
		WHERE code_hash = $1
		  AND used_count < max_uses
		  AND (expires_at IS NULL OR expires_at > now())
		  AND (bound_email IS NULL OR lower(bound_email) = $2)
		RETURNING id::text`,
		hex.EncodeToString(sum[:]),
		email,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.New(
			"INVITE_CODE_INVALID",
			"邀请码无效或已失效",
			http.StatusForbidden,
			nil,
			nil,
		)
	}
	return err
}
