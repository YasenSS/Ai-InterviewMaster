// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"github.com/google/uuid"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetInterviewReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetInterviewReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInterviewReportLogic {
	return &GetInterviewReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetInterviewReportLogic) GetInterviewReport(req *types.InterviewPath) (resp *types.InterviewReportResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err}
	var status string;err=l.svcCtx.Database.QueryRow(l.ctx,`SELECT status::text FROM interview_sessions WHERE id=$1 AND user_id=$2`,req.Id,userID).Scan(&status);if err!=nil{return nil,fmt.Errorf("interview not found: %w",err)};if status!="completed"{return nil,fmt.Errorf("interview must be completed before generating a report")}
	var reportID string;var score int;var strengthsRaw,improvementsRaw,nextRaw,qualityRaw []byte
	err=l.svcCtx.Database.QueryRow(l.ctx,`SELECT id::text,overall_score,strengths,improvements,next_steps,quality_gate FROM interview_reports WHERE session_id=$1`,req.Id).Scan(&reportID,&score,&strengthsRaw,&improvementsRaw,&nextRaw,&qualityRaw)
	if err!=nil{reportID=uuid.NewString();rows,qerr:=l.svcCtx.Database.Query(l.ctx,`SELECT id::text,ordinal,question,COALESCE(answer,'') FROM interview_turns WHERE session_id=$1 ORDER BY ordinal`,req.Id);if qerr!=nil{return nil,qerr};defer rows.Close();type row struct{id string;ordinal int;question,answer string};var turns[]row;total:=0;for rows.Next(){var t row;if err:=rows.Scan(&t.id,&t.ordinal,&t.question,&t.answer);err!=nil{return nil,err};turns=append(turns,t);n:=len([]rune(strings.TrimSpace(t.answer)));s:=40+n/3;if s>95{s=95};total+=s};if len(turns)==0{return nil,fmt.Errorf("interview has no turns")};score=total/len(turns);strengths:=[]string{"已完成全部问题","回答包含个人经历"};improvements:=[]string{"增加量化结果与技术取舍","使用 STAR 结构组织答案"};next:=[]string{"重答得分最低的问题","针对追问准备证据"};quality:=map[string]any{"passed":true,"answered_turns":len(turns),"evidence_required":true};strengthsRaw=encodeStrings(strengths);improvementsRaw=encodeStrings(improvements);nextRaw=encodeStrings(next);qualityRaw,_=json.Marshal(quality);tx,e:=l.svcCtx.Database.Begin(l.ctx);if e!=nil{return nil,e};defer tx.Rollback(l.ctx);_,e=tx.Exec(l.ctx,`INSERT INTO interview_reports(id,session_id,overall_score,strengths,improvements,next_steps,quality_gate) VALUES($1,$2,$3,$4,$5,$6,$7)`,reportID,req.Id,score,strengthsRaw,improvementsRaw,nextRaw,qualityRaw);if e!=nil{return nil,e};for _,t:=range turns{n:=len([]rune(strings.TrimSpace(t.answer)));s:=40+n/3;if s>95{s=95};critique:="回答方向正确；建议补充量化指标、约束条件和个人贡献。";golden:="建议按 STAR 展开：先交代背景与目标，再说明你的关键行动、技术取舍和可验证结果。";_,e=tx.Exec(l.ctx,`INSERT INTO interview_turn_reports(report_id,turn_id,score,critique,golden_answer,evidence) VALUES($1,$2,$3,$4,$5,$6)`,reportID,t.id,s,critique,golden,encodeStrings([]string{t.question}));if e!=nil{return nil,e}};if e=tx.Commit(l.ctx);e!=nil{return nil,e}}
	resp=&types.InterviewReportResponse{Id:reportID,SessionId:req.Id,OverallScore:score};_ = json.Unmarshal(strengthsRaw,&resp.Strengths);_ = json.Unmarshal(improvementsRaw,&resp.Improvements);_ = json.Unmarshal(nextRaw,&resp.NextSteps);var quality map[string]any;_ = json.Unmarshal(qualityRaw,&quality);resp.QualityPassed,_=quality["passed"].(bool)
	rows,err:=l.svcCtx.Database.Query(l.ctx,`SELECT it.ordinal,itr.score,itr.critique,itr.golden_answer,itr.evidence FROM interview_turn_reports itr JOIN interview_turns it ON it.id=itr.turn_id WHERE itr.report_id=$1 ORDER BY it.ordinal`,reportID);if err!=nil{return nil,err};defer rows.Close();for rows.Next(){var item types.TurnReportResponse;var evidence []byte;if err:=rows.Scan(&item.Ordinal,&item.Score,&item.Critique,&item.GoldenAnswer,&evidence);err!=nil{return nil,err};_ = json.Unmarshal(evidence,&item.Evidence);resp.Turns=append(resp.Turns,item)};return resp,rows.Err()
}
