// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnswerInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnswerInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnswerInterviewLogic {
	return &AnswerInterviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnswerInterviewLogic) AnswerInterview(req *types.AnswerInterviewRequest) (resp *types.InterviewSessionResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err};answer:=strings.TrimSpace(req.Answer);if len([]rune(answer))<5{return nil,fmt.Errorf("answer is too short")}
	tx,err:=l.svcCtx.Database.Begin(l.ctx);if err!=nil{return nil,err};defer tx.Rollback(l.ctx);var ordinal int
	err=tx.QueryRow(l.ctx,`SELECT current_ordinal FROM interview_sessions WHERE id=$1 AND user_id=$2 AND status='active' FOR UPDATE`,req.Id,userID).Scan(&ordinal);if err!=nil{return nil,fmt.Errorf("active interview not found: %w",err)}
	result,err:=tx.Exec(l.ctx,`UPDATE interview_turns SET answer=$3,answered_at=now() WHERE session_id=$1 AND ordinal=$2 AND answer IS NULL`,req.Id,ordinal,answer);if err!=nil{return nil,err};if result.RowsAffected()!=1{return nil,fmt.Errorf("current turn already answered")}
	var remaining int;_ = tx.QueryRow(l.ctx,`SELECT count(*) FROM interview_turns WHERE session_id=$1 AND ordinal>$2`,req.Id,ordinal).Scan(&remaining)
	if remaining==0{_,err=tx.Exec(l.ctx,`UPDATE interview_sessions SET status='completed',completed_at=now(),updated_at=now() WHERE id=$1`,req.Id)}else{_,err=tx.Exec(l.ctx,`UPDATE interview_sessions SET current_ordinal=$2,updated_at=now() WHERE id=$1`,req.Id,ordinal+1)};if err!=nil{return nil,err};if err:=tx.Commit(l.ctx);err!=nil{return nil,err};return loadInterview(l.ctx,l.svcCtx.Database,userID,req.Id)
}
