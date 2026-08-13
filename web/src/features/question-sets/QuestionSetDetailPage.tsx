"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { GripVertical, LockKeyhole } from "lucide-react";
import Link from "next/link";

import { Alert, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Progress, Skeleton } from "@/components/ui/Display";
import { Button } from "@/components/ui/Button";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { rememberTask } from "@/shared/lib/recent";

export function QuestionSetDetailPage({ id }: { id: string }) {
  const query = useQuery({
    queryKey: queryKeys.questionSet(id),
    queryFn: () => api.questionSet(id),
    refetchInterval: (current) => {
      if (document.visibilityState !== "visible") return false;
      return current.state.data?.status === "generating" ? cacheTimes.taskPoll : false;
    },
  });
  const retry = useMutation({
    mutationFn: async () => {
      const taskId = query.data?.task_id;
      if (!taskId) throw { code: "TASK_NOT_FOUND", message: "没有可重试的生成任务。" };
      const accepted = await api.retryTask(taskId);
      rememberTask(accepted.task_id);
      return accepted;
    },
    onSuccess: () => query.refetch(),
  });
  if (query.isPending) return <div className="page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (query.error || !query.data) return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  const item = query.data;
  const usable = item.status === "ready" || item.status === "degraded";
  return (
    <div className="page">
      <PageHeader
        eyebrow="题集详情"
        title={item.target_role || "基于简历的面试题集"}
        description={`${item.question_count} 道题 · ${item.resume.title}${item.degraded ? " · 当前为规则回退题集" : ""}`}
        action={usable ? { label: "开始模拟面试", href: `/interviews/new?question_set=${item.id}` } : undefined}
      />
      {item.status === "generating" ? <Card className="processing-state"><span className="processing-orb" /><h2>正在生成题目</h2><p>读取材料、规划能力、生成题目和质量检查正在后台进行。</p><Progress value={40} label="生成中" /></Card> : null}
      {item.status === "failed" ? (
        <Alert title="题集生成失败" tone="danger">
          {retry.error ? normalizeError(retry.error).message : "可以安全重试，不会覆盖已有面试。"}
          {item.task_id ? <div style={{ marginTop: 12 }}><Button onClick={() => retry.mutate()} loading={retry.isPending}>重试生成</Button></div> : null}
        </Alert>
      ) : null}
      {item.degraded ? <Alert title="当前为降级题集">模型不可用时使用规则题，便于本地继续训练；正式路径应重新生成。</Alert> : null}
      <div className="question-list">{item.questions.toSorted((a, b) => a.ordinal - b.ordinal).map((question) => (
        <Card className="question-card" key={question.id}>
          <div className="question-number"><GripVertical size={18} /><span>第 {question.ordinal} 题</span><Badge>{question.difficulty || "预计 3 分钟"}</Badge></div>
          <h2>{question.question}</h2>
          <div className="question-meta">
            <div><p className="eyebrow">考察意图</p><p>{question.intent}</p></div>
            <div><p className="eyebrow">期望回答点</p><ul>{question.expected_points.map((point) => <li key={point}>{point}</li>)}</ul></div>
            {question.capability_key ? <div><p className="eyebrow">能力点</p><p>{question.capability_key}</p></div> : null}
            {question.follow_up_hint ? <div><p className="eyebrow">追问提示</p><p>{question.follow_up_hint}</p></div> : null}
          </div>
        </Card>
      ))}</div>
      <Card className="disabled-actions horizontal"><LockKeyhole /><div><h2>编辑与删除暂不可用于已开面试的题集</h2><p>重新生成会创建新题集并保留历史面试。</p></div></Card>
      <Link className="back-link" href="/question-sets">返回题集列表</Link>
    </div>
  );
}
