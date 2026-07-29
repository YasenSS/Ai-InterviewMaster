"use client";

import { useQuery } from "@tanstack/react-query";
import { BriefcaseBusiness } from "lucide-react";
import Link from "next/link";

import { EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { formatDate } from "@/shared/lib/utils";

export function JobsPage() {
  const query = useQuery({ queryKey: queryKeys.jobs(), queryFn: api.jobs });
  return (
    <div className="page">
      <PageHeader eyebrow="材料库" title="职位描述" description="JD 是可选材料；添加后，题集会更贴近岗位要求。" action={{ label: "添加 JD", href: "/jobs/new" }} />
      {query.isPending ? <div className="card-grid">{[1,2,3].map((key) => <Skeleton className="skeleton-card" key={key} />)}</div> : null}
      {query.error ? <ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /> : null}
      {!query.isPending && !query.error && !query.data?.length ? <EmptyState title="还没有职位描述" description="你可以直接使用简历生成题集，也可以添加 JD，让问题更贴近目标岗位。" action={{ label: "添加第一份 JD", href: "/jobs/new" }} /> : null}
      <div className="card-grid">{query.data?.toSorted((a,b) => b.updated_at.localeCompare(a.updated_at)).map((job) => <Card className="job-card" key={job.id}><div className="card-title-row"><span className="resource-icon"><BriefcaseBusiness /></span><small>{formatDate(job.updated_at)}</small></div>{job.company ? <p className="company-name">{job.company}</p> : null}<h2>{job.title}</h2><div className="tag-list">{job.capabilities.slice(0,5).map((item) => <Badge key={item}>{item}</Badge>)}{!job.capabilities.length ? <span className="muted-copy">暂无能力标签</span> : null}</div><Link className="card-link" href={`/jobs/${job.id}`}>查看详情</Link></Card>)}</div>
    </div>
  );
}
