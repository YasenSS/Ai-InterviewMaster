"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { Award, CheckCircle2, Clock3, FileText, Printer, Target, TrendingUp } from "lucide-react";
import Link from "next/link";

import { Alert, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { api } from "@/shared/api/services";
import { formatDate, formatTimer } from "@/shared/lib/utils";

export function InterviewRecordPage({ id }: { id: string }) {
  const interview = useQuery({ queryKey: queryKeys.interview(id), queryFn: () => api.interview(id), retry: false });
  const completed = interview.data?.status === "completed";
  const report = useQuery({
    queryKey: queryKeys.report(id),
    queryFn: () => api.report(id),
    enabled: completed,
    retry: false,
    refetchInterval: (result) => {
      if (document.visibilityState !== "visible") return false;
      return ["pending", "running"].includes(result.state.data?.status ?? "") ? cacheTimes.taskPoll : false;
    },
  });
  const retryReport = useMutation({
    mutationFn: async () => {
      const taskId = report.data?.task_id;
      if (!taskId) throw { code: "TASK_NOT_FOUND", message: "没有可重试的评估任务。" };
      return api.retryTask(taskId);
    },
    onSuccess: () => report.refetch(),
  });

  if (interview.isPending) {
    return <div className="page report-page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  }
  if (interview.error || !interview.data) {
    return <div className="page"><ErrorState error={normalizeError(interview.error)} retry={() => interview.refetch()} /></div>;
  }

  const session = interview.data;
  const evaluation = report.data;
  const evaluating = completed && (report.isPending || ["pending", "running"].includes(evaluation?.status ?? ""));
  const evaluationFailed = evaluation?.status === "failed";
  const evaluationReady = Boolean(evaluation && !["pending", "running", "failed"].includes(evaluation.status));
  const visibleTurns = session.turns.filter((turn) =>
    Boolean(turn.question) && (
      completed ||
      turn.ordinal <= session.current_ordinal ||
      Boolean(turn.answer) ||
      turn.state === "answered" ||
      turn.state === "skipped"
    ),
  );
  const ordinals = Array.from(new Set([
    ...visibleTurns.map((turn) => turn.ordinal),
    ...(evaluationReady ? evaluation?.turns.map((turn) => turn.ordinal) ?? [] : []),
  ])).sort((a, b) => a - b);

  return (
    <div className="page report-page">
      <PageHeader
        eyebrow="模拟面试 · 面试记录"
        title={session.title || "后端开发模拟面试"}
        description="完整保留本场面试的模型提问、你的回答，以及逐题评分与改进答案。"
        action={<Button className="no-print" variant="secondary" onClick={() => window.print()}><Printer size={17} />打印 / 保存 PDF</Button>}
      />

      <Card className="record-overview">
        <div><FileText /><span><small>使用简历</small><strong>{session.resume.title}</strong></span></div>
        <div><Clock3 /><span><small>面试时间</small><strong>{formatDate(session.started_at ?? session.created_at)}</strong></span></div>
        <div><TrendingUp /><span><small>完整过程</small><strong>{visibleTurns.filter((turn) => Boolean(turn.answer)).length} 轮回答 · {formatTimer(session.duration_seconds)}</strong></span></div>
        <Badge tone={completed ? "success" : "warning"}>{completed ? "已完成" : "进行中"}</Badge>
      </Card>

      {!completed ? (
        <Alert title="这场面试还在进行中">
          当前记录已保存；完成面试后会在本页补充逐题评分、点评和参考答案。 <Link href={`/interviews/${id}`}>继续面试</Link>
        </Alert>
      ) : null}
      {report.error ? <Alert title="评估暂时无法加载" tone="danger">{normalizeError(report.error).message}</Alert> : null}
      {evaluating ? (
        <Card className="record-evaluation-state">
          <span className="processing-orb" />
          <div><h2>正在生成逐题评估</h2><p>完整问答已经保存。评分、点评和改进答案生成后会自动显示。</p></div>
        </Card>
      ) : null}
      {evaluationFailed ? (
        <Alert title={evaluation?.error_summary || "评估生成失败"} tone="danger">
          完整问答没有丢失。{evaluation?.task_id ? <Button variant="secondary" onClick={() => retryReport.mutate()} loading={retryReport.isPending}>重试评估</Button> : null}
        </Alert>
      ) : null}
      {retryReport.error ? <Alert title={normalizeError(retryReport.error).message} tone="danger" /> : null}
      {evaluation?.degraded ? <Alert title="当前为降级评估">完整问答已经保留，但分数仅为占位结果，不代表实际能力水平。</Alert> : null}

      {evaluationReady && evaluation ? (
        <>
          <section className="report-hero">
            <div className="score-ring"><strong>{Math.round(evaluation.overall_score)}</strong><span>总分 / 100</span></div>
            <div><span className="badge badge-success"><CheckCircle2 size={14} />评估完成</span><h2>{evaluation.quality_passed ? "整体表现达到本轮训练目标" : "建议优先补强关键回答"}</h2><p>评分用于训练复盘，不作为招聘结论。</p></div>
          </section>
          <div className="report-summary-grid">
            <Card><span className="summary-icon success"><Award /></span><h2>表现优势</h2><ul>{evaluation.strengths.length ? evaluation.strengths.map((item) => <li key={item}>{item}</li>) : <li>本轮暂无明确优势摘要。</li>}</ul></Card>
            <Card><span className="summary-icon warning"><Target /></span><h2>优先改进</h2><ul>{evaluation.improvements.length ? evaluation.improvements.map((item) => <li key={item}>{item}</li>) : <li>本轮暂无改进摘要。</li>}</ul></Card>
            <Card><span className="summary-icon brand"><TrendingUp /></span><h2>下一步训练</h2><ul>{evaluation.next_steps.length ? evaluation.next_steps.map((item) => <li key={item}>{item}</li>) : <li>可以开始下一轮模拟面试继续训练。</li>}</ul></Card>
          </div>
        </>
      ) : null}

      <section className="turn-reports interview-transcript">
        <div className="section-card-head"><div><p className="eyebrow">完整过程</p><h2>提问与回答</h2></div><span>{ordinals.length} 轮</span></div>
        {!ordinals.length ? <Card><p className="muted-copy">当前还没有已经提出的问题。</p></Card> : null}
        {ordinals.map((ordinal, index) => {
          const source = visibleTurns.find((turn) => turn.ordinal === ordinal);
          const result = evaluationReady ? evaluation?.turns.find((turn) => turn.ordinal === ordinal) : undefined;
          return (
            <Card className="turn-report" key={ordinal}>
              <div className="turn-report-head">
                <span>第 {index + 1} 轮{source?.turn_kind === "follow_up" ? " · 追问" : ""}</span>
                {result ? <strong>{Math.round(result.score)} 分</strong> : <Badge tone="neutral">{completed ? "等待评分" : "已记录"}</Badge>}
              </div>
              <div className="transcript-speaker interviewer"><span>AI 面试官</span><h2>{result?.question || source?.question || "问题内容暂不可用"}</h2></div>
              <div className="transcript-speaker candidate"><span>你的回答</span><p>{result?.answer || source?.answer || "本轮未作答"}</p></div>
              {result ? (
                <>
                  <div className="report-block accent"><h3>面试点评</h3><p>{result.critique || "暂无点评"}</p></div>
                  <div className="report-block improved-answer"><h3>参考 / 改进答案</h3><p>{result.golden_answer || "暂无参考答案"}</p></div>
                  <div className="report-block"><h3>评分依据</h3>{result.evidence.length ? <ul>{result.evidence.map((item) => <li key={item}>{item}</li>)}</ul> : <p>暂无评分依据。</p>}</div>
                </>
              ) : null}
            </Card>
          );
        })}
      </section>
    </div>
  );
}
