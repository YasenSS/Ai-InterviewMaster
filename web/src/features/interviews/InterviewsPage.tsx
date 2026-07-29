"use client";

import { GraduationCap } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { Alert, EmptyState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card } from "@/components/ui/Display";
import { Select } from "@/components/ui/Form";
import { formatDate } from "@/shared/lib/utils";
import { useRecent } from "@/shared/hooks/useRecent";

export function InterviewsPage() {
  const all = useRecent().interviews;
  const [status, setStatus] = useState("all");
  const items = all.filter((item) => status === "all" || item.status === status);
  return (
    <div className="page">
      <PageHeader eyebrow="训练记录" title="模拟面试" description="继续未完成的训练，或回看已完成面试的报告。" action={{ label: "创建面试", href: "/interviews/new" }} />
      <Alert title="面试历史接口尚未开放">当前只展示此设备最近真实创建的面试；刷新单场面试时仍以服务端状态为准。</Alert>
      <div className="filter-bar"><label>状态<Select value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">全部</option><option value="active">进行中</option><option value="completed">已完成</option></Select></label><span className="filter-count">共 {items.length} 场</span></div>
      {!items.length ? <EmptyState title={all.length ? "没有符合条件的面试" : "还没有模拟面试"} description="生成题集后即可创建一场逐题进行的模拟面试。" action={!all.length ? { label: "选择题集", href: "/question-sets" } : undefined} /> : null}
      <div className="card-grid">{items.map((item) => {
        const answered = item.turns.filter((turn) => Boolean(turn.answer)).length;
        const completed = item.status === "completed";
        return <Card className="interview-card" key={item.id}><div className="card-title-row"><span className="resource-icon"><GraduationCap /></span><Badge tone={completed ? "success" : "warning"}>{completed ? "已完成" : "进行中"}</Badge></div><h2>{item.title}</h2><p>{answered} / {item.turns.length} 题已回答</p><div className="mini-progress"><span style={{ width: `${item.turns.length ? answered / item.turns.length * 100 : 0}%` }} /></div><small>{formatDate(item.created_at)}</small><Link className="card-link" href={completed ? `/interviews/${item.id}/report` : `/interviews/${item.id}`}>{completed ? "查看报告" : "继续面试"}</Link></Card>;
      })}</div>
    </div>
  );
}
