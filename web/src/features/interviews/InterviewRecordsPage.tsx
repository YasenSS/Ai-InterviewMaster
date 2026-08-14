"use client";

import { useQuery } from "@tanstack/react-query";
import { AlertCircle, CheckCircle2, Clock3, History, LoaderCircle } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";

import { EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { Select } from "@/components/ui/Form";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { api } from "@/shared/api/services";
import { formatDate, formatTimer } from "@/shared/lib/utils";

const statusLabels: Record<string, string> = {
  active: "进行中",
  completed: "已完成",
  failed: "准备失败",
  pending: "准备中",
  preparing: "准备中",
  queued: "准备中",
  generating: "准备中",
};

function statusTone(status: string): "success" | "warning" | "danger" | "neutral" {
  if (status === "completed") return "success";
  if (status === "failed") return "danger";
  if (status === "active") return "warning";
  return "neutral";
}

export function InterviewRecordsPage() {
  const [status, setStatus] = useState("all");
  const [page, setPage] = useState(1);
  const pageSize = 20;
  const filters = { page, page_size: pageSize, status: status === "all" ? undefined : status };
  const query = useQuery({ queryKey: queryKeys.interviews(filters), queryFn: () => api.interviewPage(filters) });
  const items = useMemo(
    () => (query.data?.items ?? [])
      .toSorted((a, b) => b.created_at.localeCompare(a.created_at)),
    [query.data?.items],
  );
  const total = query.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  if (query.isPending) {
    return <div className="page"><Skeleton className="skeleton-title" /><div className="card-grid">{[1, 2, 3].map((key) => <Skeleton className="skeleton-card" key={key} />)}</div></div>;
  }
  if (query.error) {
    return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  }

  return (
    <div className="page">
      <PageHeader eyebrow="模拟面试" title="面试记录" description="继续正在进行的面试，或完整回看每一次提问、回答和评估。" action={{ label: "开始新面试", href: "/interviews/new" }} />
      <div className="filter-bar">
        <label>状态
          <Select value={status} onChange={(event) => { setStatus(event.target.value); setPage(1); }}>
            <option value="all">全部</option>
            <option value="active">进行中</option>
            <option value="completed">已完成</option>
            <option value="preparing">准备中</option>
            <option value="failed">准备失败</option>
          </Select>
        </label>
        <span className="filter-count">共 {total} 场</span>
      </div>
      {!items.length ? (
        <EmptyState
          title={total ? "这一页没有面试记录" : status !== "all" ? "没有符合条件的面试" : "还没有面试记录"}
          description={total ? "返回上一页继续查看。" : status !== "all" ? "切换状态查看其他记录。" : "选择简历、技术语言和目标公司，即可直接开始第一场面试。"}
          action={!total && status === "all" ? { label: "开始第一场面试", href: "/interviews/new" } : undefined}
        />
      ) : null}
      <div className="card-grid">
        {items.map((item) => {
          const completed = item.status === "completed";
          const failed = item.status === "failed";
          const preparing = ["pending", "preparing", "queued", "generating"].includes(item.status);
          const Icon = failed ? AlertCircle : preparing ? LoaderCircle : completed ? CheckCircle2 : Clock3;
          const href = failed ? "/interviews/new" : completed ? `/interviews/${item.id}/record` : `/interviews/${item.id}`;
          const linkLabel = failed ? "重新开始" : completed ? "查看完整记录" : preparing ? "查看准备进度" : "继续面试";
          return (
            <Card className="interview-card record-card" key={item.id}>
              <div className="card-title-row">
                <span className="resource-icon"><Icon /></span>
                <Badge tone={statusTone(item.status)}>{statusLabels[item.status] ?? item.status}</Badge>
              </div>
              <p className="record-resume">{item.resume.title}</p>
              <h2>{item.title || "后端开发模拟面试"}</h2>
              <div className="record-stats">
                <span><History size={15} />{item.answered_count} 轮已回答</span>
                {completed && item.overall_score !== undefined ? <strong>{Math.round(item.overall_score)} 分</strong> : null}
                {item.duration_seconds > 0 ? <span><Clock3 size={15} />{formatTimer(item.duration_seconds)}</span> : null}
              </div>
              <small>{formatDate(item.created_at)}</small>
              <Link className="card-link" href={href}>{linkLabel}</Link>
            </Card>
          );
        })}
      </div>
      {totalPages > 1 ? (
        <div className="filter-bar" aria-label="面试记录分页">
          <Button variant="secondary" disabled={page <= 1 || query.isFetching} onClick={() => setPage((current) => Math.max(1, current - 1))}>上一页</Button>
          <span className="filter-count">第 {page} / {totalPages} 页</span>
          <Button variant="secondary" disabled={page >= totalPages || query.isFetching} onClick={() => setPage((current) => Math.min(totalPages, current + 1))}>下一页</Button>
        </div>
      ) : null}
    </div>
  );
}
