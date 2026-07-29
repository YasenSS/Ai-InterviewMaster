"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { ClipboardCheck, FileText } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";

import { Alert, EmptyState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Display";
import { FormField, Input, Select } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { rememberQuestionSet } from "@/shared/lib/recent";

export function QuestionSetCreatePage() {
  const router = useRouter();
  const search = useSearchParams();
  const resumes = useQuery({ queryKey: queryKeys.resumes(), queryFn: api.resumes });
  const jobs = useQuery({ queryKey: queryKeys.jobs(), queryFn: api.jobs });
  const [resumeId, setResumeId] = useState("");
  const [jobId, setJobId] = useState(search.get("job") ?? "");
  const [role, setRole] = useState("");
  const mutation = useMutation({
    mutationFn: () => api.createQuestionSet({ resume_id: resumeId, job_description_id: jobId || undefined, target_role: role.trim() || undefined }),
    onSuccess: (item) => {
      rememberQuestionSet(item);
      router.push(`/question-sets/${item.id}`);
    },
  });
  const completed = resumes.data?.filter((item) => item.status === "completed") ?? [];
  const selectedResume = completed.find((item) => item.id === resumeId);
  const selectedJob = jobs.data?.find((item) => item.id === jobId);
  const error = mutation.error ? normalizeError(mutation.error) : null;
  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="题集 · 新建" title="生成定制题集" description="简历必选，JD 与目标岗位可按你的训练方向补充。" />
      {!resumes.isPending && !completed.length ? <EmptyState title="需要一份已解析简历" description="只有解析完成的简历可以用于生成题集。请先上传并等待解析完成。" action={{ label: "上传简历", href: "/resumes/new" }} /> : (
        <Card>
          <form className="form-stack" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}>
            <FormField label="简历（必选）" htmlFor="resume"><Select id="resume" value={resumeId} onChange={(event) => setResumeId(event.target.value)} required><option value="">选择已解析简历</option>{completed.map((item) => <option value={item.id} key={item.id}>{item.title}</option>)}</Select></FormField>
            <FormField label="JD（可选）" htmlFor="job" hint="不选择 JD 时，建议填写目标岗位"><Select id="job" value={jobId} onChange={(event) => setJobId(event.target.value)}><option value="">不使用 JD</option>{jobs.data?.map((item) => <option value={item.id} key={item.id}>{item.company ? `${item.company} · ` : ""}{item.title}</option>)}</Select></FormField>
            <FormField label="目标岗位（可选）" htmlFor="role" hint={!jobId && !role ? "未选择 JD，请填写目标岗位以获得更准确的问题。" : undefined}><Input id="role" value={role} onChange={(event) => setRole(event.target.value)} placeholder="例如：Go 后端工程师" maxLength={100} /></FormField>
            <div className="material-summary"><p className="eyebrow">材料摘要</p><div><FileText /><span><strong>{selectedResume?.title ?? "尚未选择简历"}</strong><small>{selectedResume?.file_name ?? "选择后显示材料信息"}</small></span></div><div><ClipboardCheck /><span><strong>{selectedJob?.title ?? (role || "通用岗位题集")}</strong><small>{selectedJob?.company ?? (jobId ? "" : "未使用 JD")}</small></span></div></div>
            <Alert title="生成时可以离开页面">题集由服务端生成。成功后会自动进入详情；请勿重复提交。</Alert>
            {error ? <Alert title={error.message} tone="danger">{error.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
            <Button type="submit" loading={mutation.isPending} disabled={!resumeId}>生成题集</Button>
          </form>
        </Card>
      )}
    </div>
  );
}
