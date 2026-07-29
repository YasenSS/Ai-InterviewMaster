"use client";

import { useQuery } from "@tanstack/react-query";
import { FileText, RefreshCw } from "lucide-react";

import { Alert, ErrorState } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { formatDate } from "@/shared/lib/utils";

const groups: Record<string, string> = { personal: "个人信息", education: "教育经历", work: "工作经历", project: "项目经历", skill: "技能", skills: "技能" };

export function ResumeDetailPage({ id }: { id: string }) {
  const query = useQuery({ queryKey: queryKeys.resume(id), queryFn: () => api.resume(id) });
  if (query.isPending) return <div className="page"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (query.error) return <div className="page"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  const resume = query.data;
  const grouped = Object.groupBy(resume.facts, (fact) => groups[fact.type.toLowerCase()] ?? "其他信息");
  return (
    <div className="page">
      <PageHeader eyebrow="简历详情" title={resume.title} description="核对解析出的事实与来源摘录，再用于后续训练。" />
      <div className="detail-grid">
        <Card className="file-summary"><span className="resource-icon large"><FileText /></span><div><strong>{resume.file_name ?? "原始文件名暂不可用"}</strong><p>版本 {resume.version_id ?? "—"}</p></div><Badge tone={resume.status === "completed" ? "success" : resume.status === "failed" ? "danger" : "warning"}>{resume.status === "completed" ? "已解析" : resume.status === "failed" ? "解析失败" : "解析中"}</Badge><dl><div><dt>上传时间</dt><dd>{formatDate(resume.created_at)}</dd></div><div><dt>更新时间</dt><dd>{formatDate(resume.updated_at)}</dd></div><div><dt>文件类型与大小</dt><dd>后端待补充</dd></div></dl></Card>
        <Card className="disabled-actions"><h2>管理简历</h2><p>重命名、重新解析和删除接口尚未开放。为了避免假成功，操作暂不可用。</p><button disabled><RefreshCw size={16} />重新解析</button><button disabled>重命名</button><button disabled>删除</button></Card>
      </div>
      {resume.status === "failed" ? <Alert title="这份简历解析失败" tone="danger">服务端尚未提供安全错误摘要。可在接口补齐后从这里重新解析。</Alert> : null}
      {resume.status !== "completed" && resume.status !== "failed" ? <Alert title="简历仍在解析中" tone="info">稍后刷新页面查看最新结果，或前往任务中心跟踪进度。</Alert> : null}
      <section className="facts-section"><div className="section-card-head"><div><p className="eyebrow">结构化事实</p><h2>解析结果</h2></div><span>{resume.facts.length} 条</span></div>
        {!resume.facts.length ? <Card><p className="muted-copy">暂时还没有可展示的结构化事实。</p></Card> : Object.entries(grouped).map(([group, facts]) => <section className="fact-group" key={group}><h3>{group}</h3><div className="fact-list">{facts?.map((fact, index) => <Card key={`${fact.key}-${index}`}><p className="fact-key">{fact.key}</p><strong>{typeof fact.value === "string" ? fact.value : JSON.stringify(fact.value)}</strong><blockquote>{fact.excerpt || "暂无来源摘录"}</blockquote></Card>)}</div></section>)}
      </section>
    </div>
  );
}
