// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"strings"
	"github.com/google/uuid"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInterviewLogic {
	return &CreateInterviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateInterviewLogic) CreateInterview(req *types.CreateInterviewRequest) (resp *types.InterviewSessionResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err};if strings.TrimSpace(req.Title)==""{req.Title="文字模拟面试"}
	tx,err:=l.svcCtx.Database.Begin(l.ctx);if err!=nil{return nil,err};defer tx.Rollback(l.ctx);sessionID:=uuid.NewString()
	result,err:=tx.Exec(l.ctx,`INSERT INTO interview_sessions(id,user_id,resume_id,question_set_id,job_description_id,title,status,current_ordinal) SELECT $1,$2,$3,qs.id,$4,$5,'active',1 FROM question_sets qs WHERE qs.id=$6 AND qs.user_id=$2 AND qs.resume_id=$3`,sessionID,userID,req.ResumeId,nullUUID(req.JobDescriptionId),req.Title,req.QuestionSetId);if err!=nil{return nil,err};if result.RowsAffected()!=1{return nil,fmt.Errorf("question set not found")}
	_,err=tx.Exec(l.ctx,`INSERT INTO interview_turns(session_id,ordinal,question) SELECT $1,ordinal,question FROM questions WHERE question_set_id=$2 ORDER BY ordinal`,sessionID,req.QuestionSetId);if err!=nil{return nil,err};if err:=tx.Commit(l.ctx);err!=nil{return nil,err};return loadInterview(l.ctx,l.svcCtx.Database,userID,sessionID)
}

func nullUUID(value string) any { if strings.TrimSpace(value)==""{return nil};return value }
