"use client";

import { ClipboardList } from "lucide-react";
import Link from "next/link";

import { Alert, EmptyState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card } from "@/components/ui/Display";
import { formatDate } from "@/shared/lib/utils";
import { useRecent } from "@/shared/hooks/useRecent";

export function QuestionSetsPage() {
  const items = useRecent().questionSets;
  return (
    <div className="page">
      <PageHeader eyebrow="训练内容" title="题集" description="查看此设备上最近真实生成的题集，并从题集开始模拟面试。" action={{ label: "生成新题集", href: "/question-sets/new" }} />
      <Alert title="题集列表接口尚未开放">当前只展示本设备本次登录期间真实生成并保存的题集，不会用虚构数据补齐历史记录。</Alert>
      {!items.length ? <EmptyState title="此设备还没有题集" description="选择一份已解析简历，并按需添加 JD 或目标岗位，即可生成定制题集。" action={{ label: "生成第一份题集", href: "/question-sets/new" }} /> : null}
      <div className="card-grid">{items.map((item) => <Card className="job-card" key={item.id}><div className="card-title-row"><span className="resource-icon"><ClipboardList /></span><Badge tone="success">已生成</Badge></div><h2>{item.target_role || "基于简历的面试题集"}</h2><p>{item.questions.length} 道题 · 简历 {item.resume_id.slice(0, 8)}…</p><small>{formatDate(item.created_at)}</small><Link className="card-link" href={`/question-sets/${item.id}`}>预览题集</Link></Card>)}</div>
    </div>
  );
}
