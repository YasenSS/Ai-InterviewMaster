"use client";

import { useQueries } from "@tanstack/react-query";
import { RefreshCcw, Sparkles } from "lucide-react";
import { useState } from "react";

import { Alert, EmptyState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Progress } from "@/components/ui/Display";
import { Select } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { useRecent } from "@/shared/hooks/useRecent";

export function TasksPage() {
  const ids = useRecent().taskIds;
  const [status, setStatus] = useState("all");
  const queries = useQueries({ queries: ids.map((id) => ({ queryKey: queryKeys.task(id), queryFn: () => api.task(id), refetchInterval: (query: { state: { data?: { status?: string } } }) => ["succeeded", "failed"].includes(query.state.data?.status ?? "") ? false : cacheTimes.taskPoll })) });
  const items = queries.map((query) => query.data).filter((item) => item && (status === "all" || item.status === status));
  return (
    <div className="page">
      <PageHeader eyebrow="异步处理" title="任务中心" description="跟踪此设备发起的简历解析任务及其真实进度。" />
      <Alert title="任务列表接口尚未开放">当前通过本设备保存的任务 ID 查询单个任务；失败重试接口开放后会在这里提供操作。</Alert>
      <div className="filter-bar"><label>状态<Select value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">全部</option><option value="pending">等待中</option><option value="running">处理中</option><option value="succeeded">成功</option><option value="failed">失败</option></Select></label></div>
      {!items.length && queries.every((query) => !query.isPending) ? <EmptyState title="没有可显示的任务" description="上传简历并启动解析后，任务会出现在这里。" action={{ label: "上传简历", href: "/resumes/new" }} /> : null}
      <div className="stack">{items.map((task) => task ? <Card className="task-card" key={task.id}><span className="resource-icon"><Sparkles /></span><div><div className="task-title"><h2>{task.type === "resume_parse" ? "简历解析" : task.type}</h2><Badge tone={task.status === "succeeded" ? "success" : task.status === "failed" ? "danger" : "warning"}>{task.status === "pending" ? "等待中" : task.status === "running" ? "处理中" : task.status === "succeeded" ? "成功" : "失败"}</Badge></div><p>任务 ID：{task.id}</p><Progress value={task.progress} label="处理进度" />{task.error ? <p className="field-error">任务未成功。服务端原始错误已隐藏。</p> : null}</div><button disabled title="重试接口尚未开放"><RefreshCcw />重试</button></Card> : null)}</div>
    </div>
  );
}
