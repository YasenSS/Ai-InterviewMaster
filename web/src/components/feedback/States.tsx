import { AlertCircle, Inbox, LockKeyhole, WifiOff } from "lucide-react";
import Link from "next/link";
import { type ReactNode } from "react";

import { Button } from "@/components/ui/Button";
import type { AppError } from "@/shared/api/client";

export function Alert({
  title,
  children,
  tone = "info",
}: {
  title: string;
  children?: ReactNode;
  tone?: "info" | "success" | "warning" | "danger";
}) {
  return (
    <div className={`alert alert-${tone}`} role={tone === "danger" ? "alert" : "status"}>
      <AlertCircle size={18} aria-hidden="true" />
      <div>
        <strong>{title}</strong>
        {children ? <div>{children}</div> : null}
      </div>
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: { label: string; href: string };
}) {
  return (
    <div className="empty-state">
      <span className="empty-icon"><Inbox aria-hidden="true" /></span>
      <h2>{title}</h2>
      <p>{description}</p>
      {action ? <Link className="button button-primary button-md" href={action.href}>{action.label}</Link> : null}
    </div>
  );
}

export function ErrorState({ error, retry }: { error: AppError; retry?: () => void }) {
  return (
    <div className="empty-state" role="alert">
      <span className="empty-icon danger"><AlertCircle aria-hidden="true" /></span>
      <h2>{error.status === 403 ? "没有访问权限" : "暂时无法加载"}</h2>
      <p>{error.message}</p>
      {error.requestId ? <small>请求 ID：{error.requestId}</small> : null}
      {retry ? <Button variant="secondary" onClick={retry}>重试</Button> : null}
    </div>
  );
}

export function BlockedState({ feature }: { feature: string }) {
  return (
    <div className="empty-state compact">
      <span className="empty-icon"><LockKeyhole aria-hidden="true" /></span>
      <h2>{feature}正在准备中</h2>
      <p>对应服务端接口尚未开放。页面结构已经就绪，当前不会模拟数据或伪造成功结果。</p>
    </div>
  );
}

export function OfflineBanner() {
  return (
    <div className="offline-banner" role="status">
      <WifiOff size={16} /> 网络已断开。未保存的面试草稿会保留在此设备。
    </div>
  );
}
