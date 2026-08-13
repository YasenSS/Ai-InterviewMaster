"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { Clock3, FileText, ListChecks } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";

import { Alert, EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, Skeleton } from "@/components/ui/Display";
import { FormField, Input, Select } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { rememberInterview } from "@/shared/lib/recent";

export function InterviewCreatePage() {
  const router = useRouter();
  const search = useSearchParams();
  const questionSets = useQuery({ queryKey: queryKeys.questionSets(), queryFn: api.questionSets });
  const usable = (questionSets.data ?? []).filter((item) => item.status === "ready" || item.status === "degraded");
  const [questionSetId, setQuestionSetId] = useState(search.get("question_set") ?? "");
  const [title, setTitle] = useState("");
  const selected = usable.find((item) => item.id === questionSetId);
  const effectiveTitle = title || (selected?.target_role ? `${selected.target_role}模拟面试` : "模拟面试");
  const mutation = useMutation({
    mutationFn: () => {
      if (!selected) throw { code: "QSET_REQUIRED", message: "请选择一个已就绪的题集。" };
      return api.createInterview({ resume_id: selected.resume_id, question_set_id: selected.id, job_description_id: selected.job_description_id, title: effectiveTitle });
    },
    onSuccess: (item) => {
      rememberInterview(item);
      router.push(`/interviews/${item.id}`);
    },
  });
  const error = mutation.error ? normalizeError(mutation.error) : null;
  if (questionSets.isPending) return <div className="page narrow-page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (questionSets.error) return <div className="page"><ErrorState error={normalizeError(questionSets.error)} retry={() => questionSets.refetch()} /></div>;
  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="模拟面试 · 新建" title="创建一场面试" description="选择题集、确认节奏，然后进入专注作答模式。" />
      {!usable.length ? <EmptyState title="还没有可使用的题集" description="请先生成题集，并等待生成完成后再开始面试。" action={{ label: "生成题集", href: "/question-sets/new" }} /> : <Card>
        <form className="form-stack" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}>
          <FormField label="题集" htmlFor="questionSet"><Select id="questionSet" value={questionSetId} onChange={(event) => { setQuestionSetId(event.target.value); setTitle(""); }} required><option value="">选择题集</option>{usable.map((item) => <option value={item.id} key={item.id}>{item.target_role || "基于简历的题集"} · {item.question_count} 题{item.degraded ? " · 降级" : ""}</option>)}</Select></FormField>
          <FormField label="面试标题" htmlFor="title"><Input id="title" value={effectiveTitle} onChange={(event) => setTitle(event.target.value)} maxLength={100} required /></FormField>
          <div className="interview-summary"><div><FileText /><span><small>关联简历</small><strong>{selected?.resume.title ?? "尚未选择"}</strong></span></div><div><ListChecks /><span><small>题目数量</small><strong>{selected?.question_count ?? 0} 道</strong></span></div><div><Clock3 /><span><small>参考时长</small><strong>{selected ? selected.question_count * 3 : 0} 分钟</strong></span></div></div>
          <Alert title="进入后按题作答">每题建议 3 分钟。全部作答后会自动完成面试并开始生成报告。</Alert>
          {error ? <Alert title={error.message} tone="danger">{error.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
          <Button type="submit" loading={mutation.isPending} disabled={!selected}>开始模拟面试</Button>
        </form>
      </Card>}
    </div>
  );
}
