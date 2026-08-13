"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { CheckCircle2, ClipboardCheck, FileText } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import { Alert, EmptyState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, Progress } from "@/components/ui/Display";
import { FormField, Input, Select } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { rememberTask } from "@/shared/lib/recent";

const stages = [
  { key: "reading_materials", label: "读取材料" },
  { key: "planning_capabilities", label: "规划能力" },
  { key: "generating_questions", label: "生成题目" },
  { key: "quality_check", label: "质量检查" },
];

export function QuestionSetCreatePage() {
  const router = useRouter();
  const search = useSearchParams();
  const resumes = useQuery({ queryKey: queryKeys.resumes(), queryFn: api.resumes });
  const jobs = useQuery({ queryKey: queryKeys.jobs(), queryFn: api.jobs });
  const [resumeId, setResumeId] = useState("");
  const [jobId, setJobId] = useState(search.get("job") ?? "");
  const [role, setRole] = useState("");
  const [taskId, setTaskId] = useState("");
  const task = useQuery({
    queryKey: queryKeys.task(taskId),
    queryFn: () => api.task(taskId),
    enabled: Boolean(taskId),
    refetchInterval: (query) => {
      if (document.visibilityState !== "visible") return false;
      return ["succeeded", "failed"].includes(query.state.data?.status ?? "") ? false : cacheTimes.taskPoll;
    },
  });
  const mutation = useMutation({
    mutationFn: () => api.createQuestionSet({ resume_id: resumeId, job_description_id: jobId || undefined, target_role: role.trim() || undefined }),
    onSuccess: (accepted) => {
      setTaskId(accepted.task_id);
      rememberTask(accepted.task_id);
    },
  });
  const retry = useMutation({
    mutationFn: () => api.retryTask(taskId),
    onSuccess: (accepted) => {
      setTaskId(accepted.task_id);
      rememberTask(accepted.task_id);
    },
  });

  useEffect(() => {
    const setId = typeof task.data?.result?.question_set_id === "string" ? task.data.result.question_set_id : "";
    if (task.data?.status === "succeeded" && setId) {
      router.replace(`/question-sets/${setId}`);
    }
  }, [router, task.data?.result, task.data?.status]);

  const completed = resumes.data?.filter((item) => item.status === "completed") ?? [];
  const selectedResume = completed.find((item) => item.id === resumeId);
  const selectedJob = jobs.data?.find((item) => item.id === jobId);
  const error = mutation.error ? normalizeError(mutation.error) : retry.error ? normalizeError(retry.error) : task.error ? normalizeError(task.error) : null;
  const stage = typeof task.data?.result?.stage === "string" ? task.data.result.stage : "";
  const stageIndex = Math.max(0, stages.findIndex((item) => item.key === stage));

  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="题集 · 新建" title="生成定制题集" description="简历必选，JD 与目标岗位可按你的训练方向补充。" />
      {!resumes.isPending && !completed.length ? <EmptyState title="需要一份已解析简历" description="只有解析完成的简历可以用于生成题集。请先上传并等待解析完成。" action={{ label: "上传简历", href: "/resumes/new" }} /> : (
        <Card>
          {!taskId ? (
            <form className="form-stack" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}>
              <FormField label="简历（必选）" htmlFor="resume"><Select id="resume" value={resumeId} onChange={(event) => setResumeId(event.target.value)} required><option value="">选择已解析简历</option>{completed.map((item) => <option value={item.id} key={item.id}>{item.title}</option>)}</Select></FormField>
              <FormField label="JD（可选）" htmlFor="job" hint="不选择 JD 时，建议填写目标岗位"><Select id="job" value={jobId} onChange={(event) => setJobId(event.target.value)}><option value="">不使用 JD</option>{jobs.data?.map((item) => <option value={item.id} key={item.id}>{item.company ? `${item.company} · ` : ""}{item.title}</option>)}</Select></FormField>
              <FormField label="目标岗位（可选）" htmlFor="role" hint={!jobId && !role ? "未选择 JD，请填写目标岗位以获得更准确的问题。" : undefined}><Input id="role" value={role} onChange={(event) => setRole(event.target.value)} placeholder="例如：Go 后端工程师" maxLength={100} /></FormField>
              <div className="material-summary"><p className="eyebrow">材料摘要</p><div><FileText /><span><strong>{selectedResume?.title ?? "尚未选择简历"}</strong><small>{selectedResume?.file_name ?? "选择后显示材料信息"}</small></span></div><div><ClipboardCheck /><span><strong>{selectedJob?.title ?? (role || "通用岗位题集")}</strong><small>{selectedJob?.company ?? (jobId ? "" : "未使用 JD")}</small></span></div></div>
              <Alert title="生成在后台进行">提交后即可离开页面。任务会继续执行，并保留在任务中心。</Alert>
              {error ? <Alert title={error.message} tone="danger">{error.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
              <Button type="submit" loading={mutation.isPending} disabled={!resumeId}>生成题集</Button>
            </form>
          ) : (
            <div className="processing-state">
              {task.data?.status === "succeeded" ? <CheckCircle2 /> : <span className="processing-orb" />}
              <h2>{task.data?.status === "failed" ? "题集生成未成功" : task.data?.status === "succeeded" ? "题集已生成" : "正在生成题集"}</h2>
              <p>{task.data?.status === "failed" ? (task.data.error?.message || "可以重试，已成功的材料读取结果会按输入哈希复用。") : "读取材料、规划能力、生成题目与质量检查都在服务端执行。"}</p>
              <ol className="stepper" aria-label="生成阶段">
                {stages.map((item, index) => (
                  <li key={item.key} className={task.data?.status === "succeeded" || index < stageIndex ? "done" : index === stageIndex ? "active" : ""}>
                    <span>{index + 1}</span>{item.label}
                  </li>
                ))}
              </ol>
              <Progress value={task.data?.progress ?? 0} label="生成进度" />
              {error || task.data?.status === "failed" ? <Alert title={error?.message ?? task.data?.error?.message ?? "生成失败，请重试。"} tone="danger">{error?.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
              {task.data?.status === "failed" ? <Button onClick={() => retry.mutate()} loading={retry.isPending}>重试生成</Button> : null}
            </div>
          )}
        </Card>
      )}
    </div>
  );
}
