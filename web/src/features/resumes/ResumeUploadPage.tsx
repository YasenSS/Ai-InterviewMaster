"use client";

import { useMutation, useQueryClient, useQuery } from "@tanstack/react-query";
import { CheckCircle2, FileUp, ShieldCheck } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { Alert } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, Progress } from "@/components/ui/Display";
import { FormField, Input } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError, uploadFile } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";

const accepted = ["application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "text/plain"];
const maxSize = 20 * 1024 * 1024;

export function ResumeUploadPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [progress, setProgress] = useState(0);
  const [taskId, setTaskId] = useState("");
  const [resumeId, setResumeId] = useState("");
  const [fileError, setFileError] = useState("");
  const task = useQuery({
    queryKey: queryKeys.task(taskId),
    queryFn: () => api.task(taskId),
    enabled: Boolean(taskId),
    refetchInterval: (query) => {
      if (document.visibilityState !== "visible") return false;
      return ["succeeded", "failed"].includes(query.state.data?.status ?? "") ? false : cacheTimes.taskPoll;
    },
  });
  const upload = useMutation({
    mutationFn: async () => {
      if (!file) throw { code: "FILE_REQUIRED", message: "请选择要上传的简历文件。" };
      const ticket = await api.createResumeUpload({
        title: title.trim() || file.name.replace(/\.[^.]+$/, ""),
        file_name: file.name,
        content_type: file.type || "text/plain",
        size_bytes: file.size,
      });
      setResumeId(ticket.resume_id);
      await uploadFile(ticket.upload_url, file, ticket.upload_headers, setProgress);
      const result = await api.completeResumeUpload(ticket.resume_id, ticket.version_id);
      setTaskId(result.task_id);
      return ticket;
    },
  });

  useEffect(() => {
    if (task.data?.status === "succeeded" && resumeId) {
      void queryClient.invalidateQueries({ queryKey: ["resumes"] });
      router.replace(`/resumes/${resumeId}`);
    }
  }, [queryClient, resumeId, router, task.data?.status]);

  useEffect(() => {
    const protect = (event: BeforeUnloadEvent) => {
      if (upload.isPending && progress < 100) event.preventDefault();
    };
    window.addEventListener("beforeunload", protect);
    return () => window.removeEventListener("beforeunload", protect);
  }, [progress, upload.isPending]);

  const choose = (next: File | null) => {
    setFileError("");
    if (!next) return setFile(null);
    const validExtension = /\.(pdf|docx|txt)$/i.test(next.name);
    if (!accepted.includes(next.type) && !validExtension) return setFileError("仅支持 PDF、DOCX 或 TXT 文件。");
    if (next.size <= 0) return setFileError("文件内容为空，请重新选择。");
    if (next.size > maxSize) return setFileError("文件不能超过 20 MiB。");
    setFile(next);
    if (!title) setTitle(next.name.replace(/\.[^.]+$/, ""));
  };
  const error = upload.error ? normalizeError(upload.error) : task.error ? normalizeError(task.error) : null;
  const stage = taskId ? "parsing" : upload.isPending ? (progress < 100 ? "uploading" : "starting") : "select";

  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="简历 · 新建" title="上传并解析简历" description="文件将直接上传到对象存储，解析完成后可核对结构化事实。" />
      <Card className="upload-card">
        <ol className="stepper" aria-label="上传步骤"><li className={stage !== "select" ? "done" : "active"}><span>1</span>选择文件</li><li className={stage === "uploading" || stage === "starting" ? "active" : stage === "parsing" ? "done" : ""}><span>2</span>安全上传</li><li className={stage === "parsing" ? "active" : ""}><span>3</span>解析内容</li></ol>
        {!taskId ? (
          <form className="form-stack" onSubmit={(event) => { event.preventDefault(); upload.mutate(); }}>
            <label className="file-drop">
              <FileUp size={30} /><strong>{file ? file.name : "选择 PDF、DOCX 或 TXT 简历"}</strong><span>{file ? `${(file.size / 1024 / 1024).toFixed(2)} MiB` : "文件大小不超过 20 MiB"}</span>
              <input type="file" accept=".pdf,.docx,.txt" onChange={(event) => choose(event.target.files?.[0] ?? null)} disabled={upload.isPending} />
            </label>
            {fileError ? <p className="field-error" role="alert">{fileError}</p> : null}
            <FormField label="简历标题" htmlFor="title" hint="用于在材料库中识别这份简历"><Input id="title" value={title} onChange={(event) => setTitle(event.target.value)} maxLength={100} /></FormField>
            {upload.isPending ? <Progress value={progress} label={progress < 100 ? "正在上传" : "正在启动解析"} /> : null}
            {error ? <Alert title={error.message} tone="danger">{error.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
            <Button type="submit" loading={upload.isPending} disabled={!file || Boolean(fileError)}>上传并开始解析</Button>
          </form>
        ) : (
          <div className="processing-state">
            {task.data?.status === "succeeded" ? <CheckCircle2 /> : <span className="processing-orb" />}
            <h2>{task.data?.status === "failed" ? "解析未成功" : task.data?.status === "succeeded" ? "解析完成" : "正在理解简历内容"}</h2>
            <p>{task.data?.status === "failed" ? "解析未成功，请重新上传文件后再试。" : "你可以离开此页面，解析会在后台继续；稍后从简历列表查看结果。"}</p>
            <Progress value={task.data?.progress ?? 0} label="解析进度" />
            {error || task.data?.status === "failed" ? <Alert title={error?.message ?? "解析失败，请稍后重试。"} tone="danger">{error?.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
          </div>
        )}
      </Card>
      <p className="privacy-note"><ShieldCheck size={17} />你的简历仅用于生成本人的训练材料。</p>
    </div>
  );
}
