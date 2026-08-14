"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Building2, Code2, FileText, Server, Sparkles } from "lucide-react";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import { Alert, EmptyState, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { FormField, Input, Select } from "@/components/ui/Form";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { api } from "@/shared/api/services";
import { rememberInterview } from "@/shared/lib/recent";

const languages = ["Java", "Go", "C++", "Python", "Rust"];
const companies = ["字节跳动", "阿里巴巴", "腾讯", "美团", "百度", "京东", "快手", "华为", "小米", "其他"];

export function InterviewCreatePage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const resumes = useQuery({ queryKey: queryKeys.resumes(), queryFn: api.resumes });
  const parsedResumes = useMemo(
    () => (resumes.data ?? []).filter((resume) => resume.status === "completed"),
    [resumes.data],
  );
  const [resumeId, setResumeId] = useState("");
  const [language, setLanguage] = useState("Java");
  const [company, setCompany] = useState("字节跳动");
  const [customCompany, setCustomCompany] = useState("");

  const effectiveResumeId = resumeId || parsedResumes[0]?.id || "";
  const targetCompany = company === "其他" ? customCompany.trim() : company;
  const mutation = useMutation({
    mutationFn: () => {
      if (!effectiveResumeId) throw { code: "RESUME_REQUIRED", message: "请选择一份已解析的简历。" };
      if (!targetCompany) throw { code: "COMPANY_REQUIRED", message: "请输入目标公司。" };
      return api.createInterview({
        resume_id: effectiveResumeId,
        primary_language: language,
        target_company: targetCompany,
      });
    },
    onSuccess: (session) => {
      rememberInterview(session);
      void queryClient.invalidateQueries({ queryKey: queryKeys.interviews() });
      router.push(`/interviews/${session.id}`);
    },
  });

  if (resumes.isPending) {
    return <div className="page narrow-page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  }
  if (resumes.error) {
    return <div className="page"><ErrorState error={normalizeError(resumes.error)} retry={() => resumes.refetch()} /></div>;
  }
  if (!parsedResumes.length) {
    return (
      <div className="page narrow-page">
        <PageHeader eyebrow="模拟面试 · 开始" title="先准备一份可用简历" description="面试会从你的真实经历出发，因此需要先完成简历解析。" />
        <EmptyState title="还没有已解析的简历" description="上传简历并等待解析完成后，即可选择技术语言和目标公司开始面试。" action={{ label: "上传简历", href: "/resumes/new" }} />
      </div>
    );
  }

  const error = mutation.error ? normalizeError(mutation.error) : null;
  return (
    <div className="page narrow-page">
      <PageHeader
        eyebrow="模拟面试 · 开始"
        title="配置本次面试"
        description="选好简历、主技术语言和目标公司，点击开始后会直接进入面试。"
      />
      <Card>
        <form className="form-stack" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}>
          <FormField label="面试简历" htmlFor="resume" hint="只展示已经解析完成的简历">
            <Select id="resume" value={effectiveResumeId} onChange={(event) => setResumeId(event.target.value)} required>
              {parsedResumes.map((resume) => <option value={resume.id} key={resume.id}>{resume.title}</option>)}
            </Select>
          </FormField>

          <FormField label="主技术语言" htmlFor="language" hint="本场面试会围绕一种语言展开，避免考察方向发散">
            <Select id="language" value={language} onChange={(event) => setLanguage(event.target.value)} required>
              {languages.map((item) => <option value={item} key={item}>{item}</option>)}
            </Select>
          </FormField>

          <FormField label="目标公司" htmlFor="company" hint="目标公司会影响考察侧重点和追问风格">
            <Select id="company" value={company} onChange={(event) => setCompany(event.target.value)} required>
              {companies.map((item) => <option value={item} key={item}>{item}</option>)}
            </Select>
          </FormField>
          {company === "其他" ? (
            <FormField label="公司名称" htmlFor="customCompany">
              <Input id="customCompany" value={customCompany} onChange={(event) => setCustomCompany(event.target.value)} maxLength={100} placeholder="输入目标公司" required />
            </FormField>
          ) : null}

          <div className="interview-setup-summary" aria-label="本次面试配置">
            <div><FileText /><span><small>简历</small><strong>{parsedResumes.find((item) => item.id === effectiveResumeId)?.title}</strong></span></div>
            <div><Code2 /><span><small>主语言</small><strong>{language}</strong></span></div>
            <div><Building2 /><span><small>目标公司</small><strong>{targetCompany || "待填写"}</strong></span></div>
            <div><Server /><span><small>岗位</small><strong>后端开发</strong></span><Badge tone="brand">首版固定</Badge></div>
          </div>

          <Alert title="面试会根据你的回答动态推进">
            模型会结合考察方向判断是继续追问、切换主题还是结束面试，不会提前向你展示后续问题。
          </Alert>
          {error ? <Alert title={error.message} tone="danger">{error.requestId ? `请求 ID：${error.requestId}` : null}</Alert> : null}
          <Button type="submit" loading={mutation.isPending} disabled={!effectiveResumeId || !targetCompany}>
            <Sparkles size={18} />开始面试
          </Button>
        </form>
      </Card>
    </div>
  );
}
