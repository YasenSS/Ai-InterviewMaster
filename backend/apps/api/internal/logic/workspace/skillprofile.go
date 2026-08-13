package workspace

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

func emptySkillProfile() *types.SkillProfileResponse {
	return &types.SkillProfileResponse{Strengths: []string{}, Gaps: []string{}, Notes: ""}
}

func loadSkillProfile(ctx context.Context, svcCtx *svc.ServiceContext, userID string) (*types.SkillProfileResponse, error) {
	response := emptySkillProfile()
	var strengths, gaps []byte
	var sourceID *string
	var updatedAt *string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT strengths, gaps, notes, source_session_id::text, to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM user_skill_profiles WHERE user_id = $1`, userID,
	).Scan(&strengths, &gaps, &response.Notes, &sourceID, &updatedAt)
	if err == pgx.ErrNoRows {
		return emptySkillProfile(), nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(strengths, &response.Strengths)
	_ = json.Unmarshal(gaps, &response.Gaps)
	if response.Strengths == nil {
		response.Strengths = []string{}
	}
	if response.Gaps == nil {
		response.Gaps = []string{}
	}
	if sourceID != nil {
		response.SourceSessionId = *sourceID
	}
	if updatedAt != nil {
		response.UpdatedAt = *updatedAt
	}
	return response, nil
}

type GetSkillProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSkillProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSkillProfileLogic {
	return &GetSkillProfileLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetSkillProfileLogic) GetSkillProfile() (*types.SkillProfileResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return loadSkillProfile(l.ctx, l.svcCtx, userID)
}

type UpdateSkillProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSkillProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSkillProfileLogic {
	return &UpdateSkillProfileLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateSkillProfileLogic) UpdateSkillProfile(req *types.UpdateSkillProfileRequest) (*types.SkillProfileResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateSkillList("strengths", req.Strengths); err != nil {
		return nil, err
	}
	if err := validateSkillList("gaps", req.Gaps); err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if utf8.RuneCountInString(notes) > 2000 {
		return nil, apperror.Validation(map[string][]string{"notes": {"长度不能超过 2000 个字符"}})
	}
	if _, err := l.svcCtx.Database.Exec(l.ctx, `
		INSERT INTO user_skill_profiles (user_id, strengths, gaps, notes, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id) DO UPDATE
		SET strengths = EXCLUDED.strengths, gaps = EXCLUDED.gaps, notes = EXCLUDED.notes, updated_at = now()`,
		userID, marshalSkillJSON(req.Strengths), marshalSkillJSON(req.Gaps), notes,
	); err != nil {
		return nil, err
	}
	return loadSkillProfile(l.ctx, l.svcCtx, userID)
}

type DeleteSkillProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteSkillProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSkillProfileLogic {
	return &DeleteSkillProfileLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteSkillProfileLogic) DeleteSkillProfile() error {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return err
	}
	_, err = l.svcCtx.Database.Exec(l.ctx, `DELETE FROM user_skill_profiles WHERE user_id = $1`, userID)
	return err
}

func validateSkillList(field string, values []string) error {
	if len(values) > 12 {
		return apperror.Validation(map[string][]string{field: {"最多 12 项"}})
	}
	for _, item := range values {
		if utf8.RuneCountInString(strings.TrimSpace(item)) > 200 {
			return apperror.Validation(map[string][]string{field: {"单项长度不能超过 200 个字符"}})
		}
	}
	return nil
}

func marshalSkillJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte("[]")
	}
	return raw
}
