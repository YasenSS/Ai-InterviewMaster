"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RefreshCcw, Sparkles } from "lucide-react";
import { useMemo, useState } from "react";

import { Alert, EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Progress, Skeleton } from "@/components/ui/Display";
import { Select } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { rememberTask } from "@/shared/lib/recent";

const typeLabel: Record<string, string> = {
  "resume.parse": "简历解析",
  "question.generate": "题集生成",
  "report.generate": "报告生成",
  "object.cleanup": "对象清理",
};

const retryable = new Set(["resume.parse", "object.cleanup", "question.generate", "report.generate"]);

export function TasksPage() {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState("all");
  const query = useQuery({
    queryKey: queryKeys.tasks(),
    queryFn: api.tasks,
    refetchInterval: (current) => {
      if (document.visibilityState !== "visible") return false;
      const items = current.state.data?.items ?? [];
      return items.some((item) => item.status === "pending" || item.status === "running") ? cacheTimes.taskPoll : false;
    },
  });
  const retry = useMutation({
    mutationFn: (id: string) => api.retryTask(id),
    onSuccess: (accepted) => {
      rememberTask(accepted.task_id);
      void queryClient.invalidateQueries({ queryKey: queryKeys.tasks() });
    },
  });
  const items = useMemo(
    () => (query.data?.items ?? []).filter((item) => status === "all" || item.status === status),
    [query.data?.items, status],
  );
  if (query.isPending) return <div className="page"><Skeleton className="skeleton-title" /><div className="stack">{[1, 2, 3].map((key) => <Skeleton className="skeleton-row" key={key} />)}</div></div>;
  if (query.error) return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  return (
    <div className="page">
      <PageHeader eyebrow="异步处理" title="任务中心" description="跟踪简历解析、题集生成和报告生成的真实进度。" />
      <div className="filter-bar"><label>状态<Select value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">全部</option><option value="pending">等待中</option><option value="running">处理中</option><option value="succeeded">成功</option><option value="failed">失败</option></Select></label></div>
      {!items.length ? <EmptyState title="没有可显示的任务" description="上传简历、生成题集或完成面试后，任务会出现在这里。" action={{ label: "上传简历", href: "/resumes/new" }} /> : null}
      <div className="stack">{items.map((task) => (
        <Card className="task-card" key={task.id}>
          <span className="resource-icon"><Sparkles /></span>
          <div>
            <div className="task-title">
              <h2>{typeLabel[task.type] ?? task.type}</h2>
              <Badge tone={task.status === "succeeded" ? "success" : task.status === "failed" ? "danger" : "warning"}>
                {task.status === "pending" ? "等待中" : task.status === "running" ? "处理中" : task.status === "succeeded" ? "成功" : "失败"}
              </Badge>
            </div>
            <p>{task.reference.title || task.reference.id}</p>
            <Progress value={task.progress} label="处理进度" />
            {task.error ? <p className="field-error">{task.error.message}</p> : null}
          </div>
          <button
            disabled={task.status !== "failed" || !retryable.has(task.type) || retry.isPending}
            onClick={() => retry.mutate(task.id)}
            title={task.status === "failed" && retryable.has(task.type) ? "重试该任务" : "当前状态不可重试"}
          >
            <RefreshCcw />重试
          </button>
        </Card>
      ))}</div>
      {retry.error ? <Alert title={normalizeError(retry.error).message} tone="danger" /> : null}
    </div>
  );
}
