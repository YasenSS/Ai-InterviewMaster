"use client";

import { useQuery } from "@tanstack/react-query";
import { GraduationCap } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";

import { EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { Select } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { formatDate } from "@/shared/lib/utils";

export function InterviewsPage() {
  const query = useQuery({ queryKey: queryKeys.interviews(), queryFn: api.interviews });
  const [status, setStatus] = useState("all");
  const items = useMemo(
    () => (query.data ?? []).filter((item) => status === "all" || item.status === status),
    [query.data, status],
  );
  if (query.isPending) return <div className="page"><Skeleton className="skeleton-title" /><div className="card-grid">{[1, 2, 3].map((key) => <Skeleton className="skeleton-card" key={key} />)}</div></div>;
  if (query.error) return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  return (
    <div className="page">
      <PageHeader eyebrow="训练记录" title="模拟面试" description="继续未完成的训练，或回看已完成面试的报告。" action={{ label: "创建面试", href: "/interviews/new" }} />
      <div className="filter-bar"><label>状态<Select value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">全部</option><option value="active">进行中</option><option value="completed">已完成</option></Select></label><span className="filter-count">共 {items.length} 场</span></div>
      {!items.length ? <EmptyState title={query.data?.length ? "没有符合条件的面试" : "还没有模拟面试"} description="生成题集后即可创建一场逐题进行的模拟面试。" action={!query.data?.length ? { label: "选择题集", href: "/question-sets" } : undefined} /> : null}
      <div className="card-grid">{items.map((item) => {
        const completed = item.status === "completed";
        const progress = item.question_count ? item.answered_count / item.question_count * 100 : 0;
        return (
          <Card className="interview-card" key={item.id}>
            <div className="card-title-row"><span className="resource-icon"><GraduationCap /></span><Badge tone={completed ? "success" : "warning"}>{completed ? "已完成" : "进行中"}</Badge></div>
            <h2>{item.title}</h2>
            <p>{item.answered_count} / {item.question_count} 题已回答</p>
            <div className="mini-progress"><span style={{ width: `${progress}%` }} /></div>
            <small>{formatDate(item.created_at)}</small>
            <Link className="card-link" href={completed ? `/interviews/${item.id}/report` : `/interviews/${item.id}`}>{completed ? "查看报告" : "继续面试"}</Link>
          </Card>
        );
      })}</div>
    </div>
  );
}
