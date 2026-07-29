"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";

import { Alert } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Display";
import { CharacterCounter, FormField, Input, Textarea } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { useToast } from "@/components/feedback/Toast";

const maxContent = 20_000;
const schema = z.object({
  company: z.string().trim().max(100, "公司名称不能超过 100 个字符"),
  title: z.string().trim().min(2, "岗位名称至少 2 个字符").max(100, "岗位名称不能超过 100 个字符"),
  content: z.string().trim().min(20, "JD 正文至少 20 个字符").max(maxContent, `JD 正文不能超过 ${maxContent} 个字符`),
});
type Values = z.infer<typeof schema>;

export function JobFormPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const form = useForm<Values>({ resolver: zodResolver(schema), mode: "onBlur", defaultValues: { company: "", title: "", content: "" } });
  const content = useWatch({ control: form.control, name: "content" });
  const mutation = useMutation({
    mutationFn: (values: Values) => api.createJob({ company: values.company || undefined, title: values.title, content: values.content }),
    onSuccess: (job) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.jobs() });
      toast("JD 已保存，能力标签已提取。");
      router.push(`/jobs/${job.id}`);
    },
  });
  const error = mutation.error ? normalizeError(mutation.error) : null;
  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="职位描述 · 新建" title="添加目标岗位" description="粘贴完整 JD，帮助题集更准确地覆盖岗位能力。" />
      <Card>
        <form className="form-stack" onSubmit={form.handleSubmit((values) => mutation.mutate(values))} noValidate>
          <FormField label="公司（可选）" htmlFor="company" error={form.formState.errors.company?.message}><Input id="company" placeholder="例如：星河科技" {...form.register("company")} /></FormField>
          <FormField label="岗位名称" htmlFor="title" error={form.formState.errors.title?.message}><Input id="title" placeholder="例如：后端开发工程师" {...form.register("title")} /></FormField>
          <FormField label="JD 正文" htmlFor="content" error={form.formState.errors.content?.message}><Textarea id="content" className="jd-textarea" placeholder="粘贴职位职责、任职要求等完整内容…" {...form.register("content")} /><CharacterCounter current={content.length} max={maxContent} /></FormField>
          {error ? <Alert title={error.message} tone="danger">{error.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
          <div className="form-actions"><Button type="button" variant="ghost" onClick={() => router.back()}>取消</Button><Button type="submit" loading={mutation.isPending}>保存并提取能力</Button></div>
        </form>
      </Card>
    </div>
  );
}
