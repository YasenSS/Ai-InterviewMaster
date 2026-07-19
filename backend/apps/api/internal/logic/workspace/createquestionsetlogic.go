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

type CreateQuestionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateQuestionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateQuestionSetLogic {
	return &CreateQuestionSetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateQuestionSetLogic) CreateQuestionSet(req *types.CreateQuestionSetRequest) (resp *types.QuestionSetResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err}
	var title,status string
	err=l.svcCtx.Database.QueryRow(l.ctx,`SELECT title,status::text FROM resumes WHERE id=$1 AND user_id=$2`,req.ResumeId,userID).Scan(&title,&status);if err!=nil{return nil,fmt.Errorf("resume not found: %w",err)}
	if status!="completed"{return nil,fmt.Errorf("resume must be parsed before generating questions")}
	setID:=uuid.NewString(); tx,err:=l.svcCtx.Database.Begin(l.ctx);if err!=nil{return nil,err};defer tx.Rollback(l.ctx)
	var jd any=nil;if strings.TrimSpace(req.JobDescriptionId)!=""{jd=req.JobDescriptionId}
	_,err=tx.Exec(l.ctx,`INSERT INTO question_sets(id,user_id,resume_id,job_description_id,target_role) VALUES($1,$2,$3,$4,NULLIF($5,''))`,setID,userID,req.ResumeId,jd,strings.TrimSpace(req.TargetRole));if err!=nil{return nil,err}
	questions:=[]types.QuestionResponse{
		{Ordinal:1,Question:"请用两分钟介绍你自己，并说明与目标岗位最匹配的经历。",Intent:"自我认知与岗位匹配",ExpectedPoints:[]string{"核心经历","量化结果","岗位关联"},FollowUpHint:"其中哪项成果最能代表你的个人贡献？"},
		{Ordinal:2,Question:"选择简历中最有挑战的项目，说明背景、目标、行动与结果。",Intent:"项目深挖",ExpectedPoints:[]string{"STAR结构","技术决策","结果指标"},FollowUpHint:"如果重新做一次，你会改变什么？"},
		{Ordinal:3,Question:"项目中遇到过什么技术故障或性能瓶颈？你如何定位？",Intent:"问题解决",ExpectedPoints:[]string{"定位路径","证据数据","复盘改进"},FollowUpHint:"如何证明修复真正有效？"},
		{Ordinal:4,Question:"描述一次你与团队成员意见不一致的经历。",Intent:"协作沟通",ExpectedPoints:[]string{"分歧背景","沟通方式","共同结果"},FollowUpHint:"你如何处理仍未达成一致的部分？"},
		{Ordinal:5,Question:"为什么申请这个岗位？未来一年希望获得什么成长？",Intent:"动机与规划",ExpectedPoints:[]string{"岗位理解","能力差距","行动计划"},FollowUpHint:"你入职前三个月会优先做什么？"},
	}
	resp=&types.QuestionSetResponse{Id:setID,ResumeId:req.ResumeId,JobDescriptionId:req.JobDescriptionId,TargetRole:req.TargetRole,Questions:questions}
	for i:=range questions{questions[i].Id=uuid.NewString();_,err=tx.Exec(l.ctx,`INSERT INTO questions(id,question_set_id,ordinal,question,intent,expected_points,follow_up_hint) VALUES($1,$2,$3,$4,$5,$6,$7)`,questions[i].Id,setID,questions[i].Ordinal,questions[i].Question,questions[i].Intent,encodeStrings(questions[i].ExpectedPoints),questions[i].FollowUpHint);if err!=nil{return nil,err}}
	resp.Questions=questions;if err:=tx.Commit(l.ctx);err!=nil{return nil,err};return resp,nil
}
