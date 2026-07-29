"use client";

import { GripVertical, LockKeyhole } from "lucide-react";
import Link from "next/link";

import { ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card } from "@/components/ui/Display";
import { useRecent } from "@/shared/hooks/useRecent";

export function QuestionSetDetailPage({ id }: { id: string }) {
  const item = useRecent().questionSets.find((value) => value.id === id);
  if (!item) return <div className="page"><ErrorState error={{ code: "NOT_FOUND", status: 404, message: "题集详情接口尚未开放，且此设备没有该题集的生成记录。" }} /></div>;
  return (
    <div className="page">
      <PageHeader eyebrow="题集详情" title={item.target_role || "基于简历的面试题集"} description={`${item.questions.length} 道题 · 按建议顺序进行`} action={{ label: "开始模拟面试", href: `/interviews/new?question_set=${item.id}` }} />
      <div className="question-list">{item.questions.toSorted((a,b) => a.ordinal-b.ordinal).map((question) => <Card className="question-card" key={question.id}><div className="question-number"><GripVertical size={18} /><span>第 {question.ordinal} 题</span><Badge>预计 3 分钟</Badge></div><h2>{question.question}</h2><div className="question-meta"><div><p className="eyebrow">考察意图</p><p>{question.intent}</p></div><div><p className="eyebrow">期望回答点</p><ul>{question.expected_points.map((point) => <li key={point}>{point}</li>)}</ul></div>{question.follow_up_hint ? <div><p className="eyebrow">追问提示</p><p>{question.follow_up_hint}</p></div> : null}</div></Card>)}</div>
      <Card className="disabled-actions horizontal"><LockKeyhole /><div><h2>编辑、删除与重新生成暂不可用</h2><p>对应后端接口未开放。重新生成后应创建新题集并保留历史面试，前端不会在本地模拟这些操作。</p></div><button disabled>编辑题集</button></Card>
      <Link className="back-link" href="/question-sets">返回题集列表</Link>
    </div>
  );
}
