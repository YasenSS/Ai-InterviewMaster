"use client";

import { useQuery } from "@tanstack/react-query";
import { ClipboardList } from "lucide-react";
import Link from "next/link";

import { EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { formatDate } from "@/shared/lib/utils";

const statusLabel: Record<string, string> = {
  generating: "生成中",
  ready: "已就绪",
  degraded: "已降级",
  failed: "失败",
  archived: "已归档",
};

export function QuestionSetsPage() {
  const query = useQuery({ queryKey: queryKeys.questionSets(), queryFn: api.questionSets });
  if (query.isPending) return <div className="page"><Skeleton className="skeleton-title" /><div className="card-grid">{[1, 2, 3].map((key) => <Skeleton className="skeleton-card" key={key} />)}</div></div>;
  if (query.error) return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  const items = query.data ?? [];
  return (
    <div className="page">
      <PageHeader eyebrow="训练内容" title="题集" description="查看已生成的题集，并从题集开始模拟面试。" action={{ label: "生成新题集", href: "/question-sets/new" }} />
      {!items.length ? <EmptyState title="还没有题集" description="选择一份已解析简历，并按需添加 JD 或目标岗位，即可生成定制题集。" action={{ label: "生成第一份题集", href: "/question-sets/new" }} /> : null}
      <div className="card-grid">{items.map((item) => {
        const usable = item.status === "ready" || item.status === "degraded";
        return (
          <Card className="job-card" key={item.id}>
            <div className="card-title-row">
              <span className="resource-icon"><ClipboardList /></span>
              <Badge tone={item.status === "failed" ? "danger" : usable ? "success" : "warning"}>{statusLabel[item.status] ?? item.status}</Badge>
            </div>
            <h2>{item.target_role || "基于简历的面试题集"}</h2>
            <p>{item.question_count} 道题 · {item.resume.title}{item.degraded ? " · 规则回退" : ""}</p>
            <small>{formatDate(item.created_at)}</small>
            <Link className="card-link" href={`/question-sets/${item.id}`}>{usable ? "预览题集" : "查看进度"}</Link>
          </Card>
        );
      })}</div>
    </div>
  );
}
