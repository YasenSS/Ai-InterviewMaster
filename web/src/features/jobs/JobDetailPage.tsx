"use client";

import { useQuery } from "@tanstack/react-query";
import { BriefcaseBusiness, LockKeyhole } from "lucide-react";
import Link from "next/link";

import { ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { formatDate } from "@/shared/lib/utils";

export function JobDetailPage({ id }: { id: string }) {
  const query = useQuery({ queryKey: queryKeys.jobs(), queryFn: api.jobs });
  if (query.isPending) return <div className="page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (query.error) return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  const job = query.data.find((item) => item.id === id);
  if (!job) return <div className="page"><ErrorState error={{ code: "NOT_FOUND", status: 404, message: "这份 JD 不存在或不属于当前账户。" }} /></div>;
  return (
    <div className="page">
      <PageHeader eyebrow={job.company || "职位描述"} title={job.title} description={`更新于 ${formatDate(job.updated_at)}`} action={{ label: "基于此 JD 生成题集", href: `/question-sets/new?job=${job.id}` }} />
      <div className="detail-grid">
        <Card className="content-card"><div className="section-card-head"><div><p className="eyebrow">岗位原文</p><h2>职位描述</h2></div><BriefcaseBusiness /></div><p className="preserve-text">{job.content}</p></Card>
        <div className="stack">
          <Card><p className="eyebrow">能力标签</p><h2>重点考察方向</h2><div className="tag-list large">{job.capabilities.map((item) => <Badge tone="brand" key={item}>{item}</Badge>)}</div>{!job.capabilities.length ? <p className="muted-copy">暂未提取出能力标签。</p> : null}</Card>
          <Card className="disabled-actions"><LockKeyhole /><h2>编辑与删除</h2><p>详情、更新和删除接口仍在后端 TODO 中。为保护历史题集一致性，当前操作不可用。</p><button disabled>编辑 JD</button><button disabled>删除 JD</button></Card>
        </div>
      </div>
      <Link className="back-link" href="/jobs">返回 JD 列表</Link>
    </div>
  );
}
