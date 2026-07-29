"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowRight, BriefcaseBusiness, ClipboardList, FileText, GraduationCap, Sparkles } from "lucide-react";
import Link from "next/link";

import { Alert, ErrorState } from "@/components/feedback/States";
import { Card, Skeleton } from "@/components/ui/Display";
import { PageHeader } from "@/components/layout/PageHeader";
import { api } from "@/shared/api/services";
import { queryKeys } from "@/shared/api/query";
import { normalizeError } from "@/shared/api/client";
import { formatDate } from "@/shared/lib/utils";
import { getRecent } from "@/shared/lib/recent";
import { useAuth } from "@/features/auth/AuthGate";

export function DashboardPage() {
  const { user } = useAuth();
  const resumes = useQuery({ queryKey: queryKeys.resumes(), queryFn: api.resumes });
  const jobs = useQuery({ queryKey: queryKeys.jobs(), queryFn: api.jobs });
  const recent = getRecent();

  if (resumes.isPending || jobs.isPending) return <DashboardSkeleton />;
  if (resumes.error) return <ErrorState error={normalizeError(resumes.error)} retry={() => resumes.refetch()} />;

  const hasMaterials = Boolean(resumes.data?.length || jobs.data?.length);
  const interviews = recent.interviews;
  return (
    <div className="page">
      <PageHeader
        eyebrow="训练工作台"
        title={`${user?.display_name ? `${user.display_name}，` : ""}准备好继续了吗？`}
        description="从真实材料开始，完成一轮有节奏、可复盘的面试训练。"
        action={{ label: "开始新训练", href: resumes.data?.some((item) => item.status === "completed") ? "/question-sets/new" : "/resumes/new" }}
      />
      {!hasMaterials ? (
        <Card className="onboarding-card">
          <span className="onboarding-icon"><Sparkles /></span>
          <div><p className="eyebrow">第一次使用</p><h2>先上传一份简历</h2><p>我们会从你的真实经历中提取结构化事实，作为后续题集和面试的材料。</p></div>
          <Link className="button button-primary button-md" href="/resumes/new">上传简历 <ArrowRight size={17} /></Link>
        </Card>
      ) : null}
      <section className="metric-grid" aria-label="真实资源统计">
        <Card><span className="metric-icon"><FileText /></span><p>简历</p><strong>{resumes.data?.length ?? 0}</strong><small>已保存材料</small></Card>
        <Card><span className="metric-icon"><BriefcaseBusiness /></span><p>职位描述</p><strong>{jobs.data?.length ?? 0}</strong><small>可选训练材料</small></Card>
        <Card><span className="metric-icon"><GraduationCap /></span><p>本设备最近面试</p><strong>{interviews.length}</strong><small>正式列表接口待开放</small></Card>
        <Card><span className="metric-icon"><ClipboardList /></span><p>本设备最近题集</p><strong>{recent.questionSets.length}</strong><small>正式列表接口待开放</small></Card>
      </section>
      <div className="dashboard-grid">
        <section className="section-card">
          <div className="section-card-head"><div><p className="eyebrow">最近材料</p><h2>继续准备</h2></div><Link href="/resumes">查看全部</Link></div>
          {(resumes.data ?? []).slice(0, 4).map((resume) => (
            <Link className="resource-row" href={`/resumes/${resume.id}`} key={resume.id}>
              <span className="resource-icon"><FileText /></span>
              <span><strong>{resume.title}</strong><small>{resume.file_name ?? "简历文件"} · {formatDate(resume.updated_at)}</small></span>
              <span className={`status-text status-${resume.status}`}>{resume.status === "completed" ? "已解析" : resume.status === "failed" ? "失败" : "处理中"}</span>
            </Link>
          ))}
          {!resumes.data?.length ? <p className="muted-copy">还没有简历。上传后会在这里显示。</p> : null}
        </section>
        <section className="section-card">
          <div className="section-card-head"><div><p className="eyebrow">快捷入口</p><h2>训练资源</h2></div></div>
          <div className="quick-grid">
            <Link href="/jobs/new"><BriefcaseBusiness /><span><strong>添加 JD</strong><small>让题目更贴近岗位</small></span></Link>
            <Link href="/question-sets/new"><ClipboardList /><span><strong>生成题集</strong><small>组合简历与目标方向</small></span></Link>
            <Link href="/interviews/new"><GraduationCap /><span><strong>创建面试</strong><small>从已有题集开始</small></span></Link>
            <Link href="/tasks"><Sparkles /><span><strong>任务中心</strong><small>跟踪简历解析进度</small></span></Link>
          </div>
        </section>
      </div>
      <Alert title="统计摘要正在准备中">平均分、趋势和高频改进方向依赖仪表盘聚合接口；当前只展示真实可核验的资源数量。</Alert>
    </div>
  );
}

function DashboardSkeleton() {
  return <div className="page"><Skeleton className="skeleton-title" /><div className="metric-grid">{[1,2,3,4].map((key) => <Skeleton className="skeleton-card" key={key} />)}</div></div>;
}
