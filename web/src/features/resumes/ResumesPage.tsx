"use client";

import { useQuery } from "@tanstack/react-query";
import { FileText, RotateCcw } from "lucide-react";
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

const statusLabels: Record<string, string> = {
  draft: "草稿", uploading: "上传中", pending: "等待解析", processing: "解析中", completed: "已解析", failed: "解析失败",
};
const statusTone = (status: string) => status === "completed" ? "success" : status === "failed" ? "danger" : "warning";

export function ResumesPage() {
  const query = useQuery({ queryKey: queryKeys.resumes(), queryFn: api.resumes });
  const [status, setStatus] = useState("all");
  const [sort, setSort] = useState("desc");
  const items = useMemo(() => {
    const filtered = (query.data ?? []).filter((item) => status === "all" || item.status === status);
    return filtered.toSorted((a, b) => (sort === "desc" ? b.updated_at.localeCompare(a.updated_at) : a.updated_at.localeCompare(b.updated_at)));
  }, [query.data, sort, status]);

  return (
    <div className="page">
      <PageHeader eyebrow="材料库" title="简历" description="管理用于生成题集和模拟面试的真实经历材料。" action={{ label: "上传简历", href: "/resumes/new" }} />
      <div className="filter-bar" aria-label="简历筛选">
        <label>解析状态<Select value={status} onChange={(e) => setStatus(e.target.value)}><option value="all">全部状态</option><option value="completed">已解析</option><option value="processing">解析中</option><option value="pending">等待解析</option><option value="failed">解析失败</option></Select></label>
        <label>更新时间<Select value={sort} onChange={(e) => setSort(e.target.value)}><option value="desc">从新到旧</option><option value="asc">从旧到新</option></Select></label>
        <span className="filter-count">共 {items.length} 份</span>
      </div>
      {query.isPending ? <div className="stack">{[1,2,3].map((key) => <Skeleton className="skeleton-row" key={key} />)}</div> : null}
      {query.error ? <ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /> : null}
      {!query.isPending && !query.error && !items.length ? <EmptyState title={status === "all" ? "还没有简历" : "没有符合筛选条件的简历"} description={status === "all" ? "上传简历后，我们会解析其中的经历事实，用于定制题集。" : "尝试切换解析状态查看其他简历。"} action={status === "all" ? { label: "上传第一份简历", href: "/resumes/new" } : undefined} /> : null}
      {items.length ? (
        <div className="responsive-list">
          <div className="desktop-table" role="table" aria-label="简历列表">
            <div className="table-head" role="row"><span>简历</span><span>状态</span><span>更新时间</span><span className="sr-only">操作</span></div>
            {items.map((item) => <Link className="table-row" role="row" href={`/resumes/${item.id}`} key={item.id}><span className="title-cell"><span className="resource-icon"><FileText /></span><span><strong>{item.title}</strong><small>{item.file_name ?? "原始文件名暂不可用"}</small></span></span><span><Badge tone={statusTone(item.status)}>{statusLabels[item.status] ?? "未知状态"}</Badge></span><span>{formatDate(item.updated_at)}</span><span className="row-link">查看</span></Link>)}
          </div>
          <div className="mobile-cards">{items.map((item) => <Card key={item.id}><div className="card-title-row"><span className="resource-icon"><FileText /></span><Badge tone={statusTone(item.status)}>{statusLabels[item.status] ?? "未知状态"}</Badge></div><h2>{item.title}</h2><p>{item.file_name ?? "原始文件名暂不可用"}</p><small>{formatDate(item.updated_at)}</small><Link className="card-link" href={`/resumes/${item.id}`}>查看详情</Link></Card>)}</div>
        </div>
      ) : null}
      {items.some((item) => item.status === "failed") ? <p className="inline-note"><RotateCcw size={16} />重新解析接口尚未开放；失败原因会在详情页安全展示。</p> : null}
    </div>
  );
}
