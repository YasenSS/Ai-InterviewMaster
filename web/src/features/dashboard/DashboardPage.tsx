"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowRight, CheckCircle2, FileText, GraduationCap, History, PlayCircle, Timer } from "lucide-react";
import Link from "next/link";

import { ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Card, Skeleton } from "@/components/ui/Display";
import { useAuth } from "@/features/auth/AuthGate";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { api } from "@/shared/api/services";
import { formatDate } from "@/shared/lib/utils";
import { getRecent } from "@/shared/lib/recent";

export function DashboardPage() {
  const { user } = useAuth();
  const resumes = useQuery({ queryKey: queryKeys.resumes(), queryFn: api.resumes });
  const interviews = useQuery({ queryKey: queryKeys.interviews(), queryFn: api.interviews });
  const recent = getRecent();

  if (resumes.isPending) return <DashboardSkeleton />;
  if (resumes.error) return <ErrorState error={normalizeError(resumes.error)} retry={() => resumes.refetch()} />;

  const interviewItems = interviews.data ?? recent.interviews;
  const completedCount = interviewItems.filter((item) => item.status === "completed").length;
  const activeCount = interviewItems.filter((item) => item.status === "active").length;
  const hasResume = Boolean(resumes.data?.some((item) => item.status === "completed"));

  return (
    <div className="page">
      <PageHeader
        eyebrow="训练工作台"
        title={`${user?.display_name ? `${user.display_name}，` : ""}准备好开始了吗？`}
        description="从真实简历出发，完成一轮会动态追问、能够完整复盘的模拟面试。"
        action={{ label: "开始新面试", href: "/interviews/new" }}
      />

      {!hasResume ? (
        <Card className="onboarding-card">
          <span className="onboarding-icon"><FileText /></span>
          <div><p className="eyebrow">开始之前</p><h2>先上传一份简历</h2><p>面试官会从你的真实经历中寻找考察线索。解析完成后，只需选择技术语言和目标公司即可开场。</p></div>
          <Link className="button button-primary button-md" href="/resumes/new">上传简历<ArrowRight size={17} /></Link>
        </Card>
      ) : null}

      <section className="metric-grid" aria-label="训练统计">
        <Card><span className="metric-icon"><FileText /></span><p>简历</p><strong>{resumes.data?.length ?? 0}</strong><small>已保存材料</small></Card>
        <Card><span className="metric-icon"><GraduationCap /></span><p>全部面试</p><strong>{interviewItems.length}</strong><small>累计训练场次</small></Card>
        <Card><span className="metric-icon"><CheckCircle2 /></span><p>已完成</p><strong>{completedCount}</strong><small>可完整复盘</small></Card>
        <Card><span className="metric-icon"><Timer /></span><p>进行中</p><strong>{activeCount}</strong><small>可以继续作答</small></Card>
      </section>

      <div className="dashboard-grid">
        <section className="section-card">
          <div className="section-card-head"><div><p className="eyebrow">最近简历</p><h2>训练材料</h2></div><Link href="/resumes">查看全部</Link></div>
          {(resumes.data ?? []).slice(0, 4).map((resume) => (
            <Link className="resource-row" href={`/resumes/${resume.id}`} key={resume.id}>
              <span className="resource-icon"><FileText /></span>
              <span><strong>{resume.title}</strong><small>{resume.file_name ?? "简历文件"} · {formatDate(resume.updated_at)}</small></span>
              <span className={`status-text status-${resume.status}`}>{resume.status === "completed" ? "已解析" : resume.status === "failed" ? "失败" : "处理中"}</span>
            </Link>
          ))}
          {!resumes.data?.length ? <p className="muted-copy">还没有简历。上传并解析后即可用于模拟面试。</p> : null}
        </section>

        <section className="section-card">
          <div className="section-card-head"><div><p className="eyebrow">面试入口</p><h2>下一步</h2></div></div>
          <div className="quick-grid training-quick-grid">
            <Link href="/interviews/new"><PlayCircle /><span><strong>开始面试</strong><small>选择语言和目标公司后直接开场</small></span></Link>
            <Link href="/interviews/records"><History /><span><strong>面试记录</strong><small>回看完整问答、评分和改进答案</small></span></Link>
            <Link href="/resumes/new"><FileText /><span><strong>上传新简历</strong><small>为不同经历准备新的训练材料</small></span></Link>
          </div>
        </section>
      </div>
    </div>
  );
}

function DashboardSkeleton() {
  return <div className="page"><Skeleton className="skeleton-title" /><div className="metric-grid">{[1, 2, 3, 4].map((key) => <Skeleton className="skeleton-card" key={key} />)}</div></div>;
}
