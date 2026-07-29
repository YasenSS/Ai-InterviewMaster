"use client";

import { useTheme } from "next-themes";
import { LogOut, Moon, Shield, Sun, UserRound } from "lucide-react";

import { Alert } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Display";
import { Input } from "@/components/ui/Form";
import { useAuth } from "@/features/auth/AuthGate";
import { useHydrated } from "@/shared/hooks/useRecent";

export function SettingsPage() {
  const { user, logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const mounted = useHydrated();
  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="账户偏好" title="设置" description="管理个人资料、外观和账户登录状态。" />
      <section className="settings-section"><div className="settings-heading"><UserRound /><div><h2>个人资料</h2><p>资料写入接口尚未开放，当前为只读。</p></div></div><Card><div className="settings-fields"><label>显示名称<Input value={user?.display_name ?? ""} readOnly /></label><label>邮箱<Input value={user?.email ?? ""} readOnly /></label></div></Card></section>
      <section className="settings-section"><div className="settings-heading"><Sun /><div><h2>外观</h2><p>选择浅色、深色，或跟随系统设置。</p></div></div><Card>{mounted ? <div className="theme-options" role="radiogroup" aria-label="主题"><button className={theme === "light" ? "active" : ""} onClick={() => setTheme("light")} role="radio" aria-checked={theme === "light"}><Sun />浅色</button><button className={theme === "dark" ? "active" : ""} onClick={() => setTheme("dark")} role="radio" aria-checked={theme === "dark"}><Moon />深色</button><button className={theme === "system" ? "active" : ""} onClick={() => setTheme("system")} role="radio" aria-checked={theme === "system"}><span className="system-icon">A</span>跟随系统</button></div> : null}</Card></section>
      <section className="settings-section"><div className="settings-heading"><Shield /><div><h2>安全</h2><p>修改密码接口尚未开放。</p></div></div><Card><Alert title="暂时无法修改密码">后端完成身份验证与修改密码接口后，此处将提供当前密码、新密码和确认密码表单。</Alert></Card></section>
      <section className="settings-section"><div className="settings-heading"><LogOut /><div><h2>账户</h2><p>退出会清理本设备的访问令牌、查询缓存、任务记录和面试草稿索引。</p></div></div><Card className="logout-card"><div><h3>退出当前账户</h3><p>后端正式退出与 Refresh Cookie 撤销接口尚未开放；当前仅执行完整客户端清理。</p></div><Button variant="danger" onClick={logout}><LogOut />退出登录</Button></Card></section>
    </div>
  );
}
