"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { Award, CheckCircle2, Printer, Target, TrendingUp } from "lucide-react";
import Link from "next/link";

import { Alert, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, Progress, Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { rememberTask } from "@/shared/lib/recent";

export function ReportPage({ id }: { id: string }) {
  const report = useQuery({
    queryKey: queryKeys.report(id),
    queryFn: () => api.report(id),
    retry: false,
    refetchInterval: (query) => {
      if (document.visibilityState !== "visible") return false;
      return ["pending", "running"].includes(query.state.data?.status ?? "") ? cacheTimes.taskPoll : false;
    },
  });
  const interview = useQuery({ queryKey: queryKeys.interview(id), queryFn: () => api.interview(id), retry: false });
  const retry = useMutation({
    mutationFn: async () => {
      const taskId = report.data?.task_id;
      if (!taskId) throw { code: "TASK_NOT_FOUND", message: "没有可重试的报告任务。" };
      const accepted = await api.retryTask(taskId);
      rememberTask(accepted.task_id);
      return accepted;
    },
    onSuccess: () => report.refetch(),
  });
  if (report.isPending) return <div className="page report-page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (report.error) {
    const error = normalizeError(report.error);
    const unfinished = error.status === 409;
    return <div className="page"><ErrorState error={{ ...error, message: unfinished ? "面试尚未完成，完成全部回答后才能生成报告。" : error.message }} retry={unfinished ? undefined : () => report.refetch()} />{unfinished ? <Link className="button button-primary button-md centered-action" href={`/interviews/${id}`}>返回面试</Link> : null}</div>;
  }
  const data = report.data;
  if (data.status === "pending" || data.status === "running") {
    return (
      <div className="page report-page">
        <PageHeader eyebrow="训练复盘" title="正在生成报告" description="评分与报告在后台完成，你可以离开此页面稍后回来。" />
        <Card className="processing-state">
          <span className="processing-orb" />
          <h2>{data.status === "running" ? "正在评测回答" : "报告任务排队中"}</h2>
          <p>空回答会记为 0 分且不调用模型。完成后会展示各题证据与训练建议。</p>
          <Progress value={data.status === "running" ? 55 : 15} label="报告进度" />
        </Card>
      </div>
    );
  }
  if (data.status === "failed") {
    return (
      <div className="page report-page">
        <PageHeader eyebrow="训练复盘" title="报告生成失败" description="已完成的面试和回答不受影响，可以安全重试。" />
        <Alert title={data.error_summary || "报告生成失败，请重试。"} tone="danger" />
        {data.task_id ? <Button onClick={() => retry.mutate()} loading={retry.isPending}>重试生成报告</Button> : null}
        {retry.error ? <Alert title={normalizeError(retry.error).message} tone="danger" /> : null}
      </div>
    );
  }
  return (
    <div className="page report-page">
      <PageHeader eyebrow="训练复盘" title="面试报告" description="从每一道题的反馈中，找到下一轮训练最值得投入的方向。" action={<Button className="no-print" variant="secondary" onClick={() => window.print()}><Printer size={17} />打印 / 保存 PDF</Button>} />
      {data.degraded ? <Alert title="当前为降级报告">未调用评分模型，仅记录作答完整性。分数用于占位，不代表能力评估。</Alert> : null}
      <section className="report-hero"><div className="score-ring"><strong>{Math.round(data.overall_score)}</strong><span>总分 / 100</span></div><div><span className="badge badge-success"><CheckCircle2 size={14} />{data.degraded ? "降级完成" : "评估完成"}</span><h2>{data.quality_passed ? "整体表现达到本轮训练目标" : "已完成评估，建议优先补强关键回答"}</h2><p>评分为训练建议，不作为招聘结论。报告中的事实应能追溯到材料或回答。</p></div></section>
      <div className="report-summary-grid">
        <Card><span className="summary-icon success"><Award /></span><h2>表现优势</h2><ul>{data.strengths.length ? data.strengths.map((item) => <li key={item}>{item}</li>) : <li>本轮暂无明确优势摘要。</li>}</ul></Card>
        <Card><span className="summary-icon warning"><Target /></span><h2>优先改进</h2><ul>{data.improvements.length ? data.improvements.map((item) => <li key={item}>{item}</li>) : <li>本轮暂无改进摘要。</li>}</ul></Card>
        <Card><span className="summary-icon brand"><TrendingUp /></span><h2>下一步训练</h2><ul>{data.next_steps.length ? data.next_steps.map((item) => <li key={item}>{item}</li>) : <li>可使用同一材料开始下一轮训练。</li>}</ul></Card>
      </div>
      <section className="score-chart"><div className="section-card-head"><div><p className="eyebrow">各题表现</p><h2>得分分布</h2></div></div>{data.turns.map((turn) => <div className="bar-row" key={turn.ordinal}><span>第 {turn.ordinal} 题</span><div><i style={{ width: `${Math.max(0, Math.min(100, turn.score))}%` }} /></div><strong>{turn.score} 分</strong></div>)}</section>
      <section className="turn-reports"><div className="section-card-head"><div><p className="eyebrow">逐题分析</p><h2>回答与反馈</h2></div></div>{data.turns.map((turn) => {
        const source = interview.data?.turns.find((item) => item.ordinal === turn.ordinal);
        return <Card className="turn-report" key={turn.ordinal}><div className="turn-report-head"><span>第 {turn.ordinal} 题</span><strong>{turn.score} 分</strong></div><h2>{turn.question || source?.question || "后端报告暂未返回原问题"}</h2><div className="report-block"><h3>你的回答</h3><p>{turn.answer || source?.answer || "本题未作答"}</p></div><div className="report-block accent"><h3>点评</h3><p>{turn.critique}</p></div><div className="report-block"><h3>参考回答</h3><p>{turn.golden_answer}</p></div><div className="report-block"><h3>评价证据</h3>{turn.evidence.length ? <ul>{turn.evidence.map((item) => <li key={item}>{item}</li>)}</ul> : <p>暂无证据条目。</p>}</div></Card>;
      })}</section>
      {interview.error ? <Alert title="原问题与回答暂未加载">评分和点评来自报告接口；原题与原回答加载失败，可重试刷新页面。</Alert> : null}
    </div>
  );
}
