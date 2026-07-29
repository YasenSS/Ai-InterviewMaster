"use client";

import { useQuery } from "@tanstack/react-query";
import { Award, CheckCircle2, Printer, Target, TrendingUp } from "lucide-react";
import Link from "next/link";

import { Alert, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";

export function ReportPage({ id }: { id: string }) {
  const report = useQuery({ queryKey: queryKeys.report(id), queryFn: () => api.report(id), retry: false });
  const interview = useQuery({ queryKey: queryKeys.interview(id), queryFn: () => api.interview(id), retry: false });
  if (report.isPending) return <div className="page report-page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (report.error) {
    const error = normalizeError(report.error);
    const unfinished = error.status === 409;
    return <div className="page"><ErrorState error={{ ...error, message: unfinished ? "面试尚未完成，完成全部回答后才能生成报告。" : error.message }} retry={unfinished ? undefined : () => report.refetch()} />{unfinished ? <Link className="button button-primary button-md centered-action" href={`/interviews/${id}`}>返回面试</Link> : null}</div>;
  }
  const data = report.data;
  return (
    <div className="page report-page">
      <PageHeader eyebrow="训练复盘" title="面试报告" description="从每一道题的反馈中，找到下一轮训练最值得投入的方向。" action={<Button className="no-print" variant="secondary" onClick={() => window.print()}><Printer size={17} />打印 / 保存 PDF</Button>} />
      <section className="report-hero"><div className="score-ring"><strong>{Math.round(data.overall_score)}</strong><span>总分 / 100</span></div><div><span className="badge badge-success"><CheckCircle2 size={14} />评估完成</span><h2>{data.quality_passed ? "整体表现达到本轮训练目标" : "已完成评估，建议优先补强关键回答"}</h2><p>报告基于本场真实回答生成。分数用于训练反馈，不代表实际面试结果。</p></div></section>
      <div className="report-summary-grid">
        <Card><span className="summary-icon success"><Award /></span><h2>表现优势</h2><ul>{data.strengths.length ? data.strengths.map((item) => <li key={item}>{item}</li>) : <li>本轮暂无明确优势摘要。</li>}</ul></Card>
        <Card><span className="summary-icon warning"><Target /></span><h2>优先改进</h2><ul>{data.improvements.length ? data.improvements.map((item) => <li key={item}>{item}</li>) : <li>本轮暂无改进摘要。</li>}</ul></Card>
        <Card><span className="summary-icon brand"><TrendingUp /></span><h2>下一步训练</h2><ul>{data.next_steps.length ? data.next_steps.map((item) => <li key={item}>{item}</li>) : <li>可使用同一材料开始下一轮训练。</li>}</ul></Card>
      </div>
      <section className="score-chart"><div className="section-card-head"><div><p className="eyebrow">各题表现</p><h2>得分分布</h2></div></div>{data.turns.map((turn) => <div className="bar-row" key={turn.ordinal}><span>第 {turn.ordinal} 题</span><div><i style={{ width: `${Math.max(0, Math.min(100, turn.score))}%` }} /></div><strong>{turn.score} 分</strong></div>)}</section>
      <section className="turn-reports"><div className="section-card-head"><div><p className="eyebrow">逐题分析</p><h2>回答与反馈</h2></div></div>{data.turns.map((turn) => {
        const source = interview.data?.turns.find((item) => item.ordinal === turn.ordinal);
        return <Card className="turn-report" key={turn.ordinal}><div className="turn-report-head"><span>第 {turn.ordinal} 题</span><strong>{turn.score} 分</strong></div><h2>{source?.question ?? "后端报告暂未返回原问题"}</h2><div className="report-block"><h3>你的回答</h3><p>{source?.answer || "本题未作答"}</p></div><div className="report-block accent"><h3>点评</h3><p>{turn.critique}</p></div><div className="report-block"><h3>参考回答</h3><p>{turn.golden_answer}</p></div><div className="report-block"><h3>评价证据</h3>{turn.evidence.length ? <ul>{turn.evidence.map((item) => <li key={item}>{item}</li>)}</ul> : <p>暂无证据条目。</p>}</div></Card>;
      })}</section>
      {interview.error ? <Alert title="原问题与回答暂未加载">评分和点评来自报告接口；原题与原回答加载失败，可重试刷新页面。</Alert> : null}
    </div>
  );
}
