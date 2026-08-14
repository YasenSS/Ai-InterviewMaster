"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTheme } from "next-themes";
import { Download, LogOut, Moon, Shield, Sun, Trash2, UserRound } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Alert } from "@/components/feedback/States";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Display";
import { Input, Textarea } from "@/components/ui/Form";
import { useAuth } from "@/features/auth/AuthGate";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { useHydrated } from "@/shared/hooks/useRecent";

export function SettingsPage() {
  const { user, logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const mounted = useHydrated();
  const queryClient = useQueryClient();
  const [displayName, setDisplayName] = useState(user?.display_name ?? "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [strengths, setStrengths] = useState("");
  const [gaps, setGaps] = useState("");
  const [notes, setNotes] = useState("");

  const profileQuery = useQuery({
    queryKey: queryKeys.skillProfile,
    queryFn: api.skillProfile,
  });

  useEffect(() => {
    if (!user?.display_name) return;
    const timer = window.setTimeout(() => setDisplayName(user.display_name), 0);
    return () => window.clearTimeout(timer);
  }, [user?.display_name]);

  useEffect(() => {
    if (!profileQuery.data) return;
    const timer = window.setTimeout(() => {
      setStrengths(profileQuery.data?.strengths.join("\n") ?? "");
      setGaps(profileQuery.data?.gaps.join("\n") ?? "");
      setNotes(profileQuery.data?.notes ?? "");
    }, 0);
    return () => window.clearTimeout(timer);
  }, [profileQuery.data]);

  const saveName = useMutation({
    mutationFn: () => api.updateMe({ display_name: displayName.trim() }),
    onSuccess: (next) => {
      queryClient.setQueryData(queryKeys.me, next);
    },
  });
  const savePassword = useMutation({
    mutationFn: () => api.changePassword({ current_password: currentPassword, new_password: newPassword }),
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
    },
  });
  const saveProfile = useMutation({
    mutationFn: () => api.updateSkillProfile({
      strengths: strengths.split("\n").map((item) => item.trim()).filter(Boolean),
      gaps: gaps.split("\n").map((item) => item.trim()).filter(Boolean),
      notes,
    }),
    onSuccess: (next) => queryClient.setQueryData(queryKeys.skillProfile, next),
  });
  const clearProfile = useMutation({
    mutationFn: api.deleteSkillProfile,
    onSuccess: () => {
      setStrengths("");
      setGaps("");
      setNotes("");
      queryClient.invalidateQueries({ queryKey: queryKeys.skillProfile });
    },
  });
  const exportMe = useMutation({
    mutationFn: api.exportMe,
    onSuccess: (payload) => {
      const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "interviewmaster-export.json";
      link.click();
      URL.revokeObjectURL(url);
    },
  });
  const deleteMe = useMutation({
    mutationFn: () => api.deleteMe(deletePassword),
    onSuccess: () => logout(),
  });

  return (
    <div className="page narrow-page">
      <PageHeader eyebrow="账户偏好" title="设置" description="管理个人资料、能力画像、隐私和登录状态。" />
      <section className="settings-section">
        <div className="settings-heading"><UserRound /><div><h2>个人资料</h2><p>显示名称会用于面试报告抬头。</p></div></div>
        <Card>
          <div className="settings-fields">
            <label>显示名称<Input value={displayName} onChange={(event) => setDisplayName(event.target.value)} /></label>
            <label>邮箱<Input value={user?.email ?? ""} readOnly /></label>
          </div>
          {saveName.error ? <Alert title={normalizeError(saveName.error).message} tone="danger" /> : null}
          <Button onClick={() => saveName.mutate()} loading={saveName.isPending} disabled={!displayName.trim()}>保存资料</Button>
        </Card>
      </section>
      <section className="settings-section">
        <div className="settings-heading"><Sun /><div><h2>外观</h2><p>选择浅色、深色，或跟随系统设置。</p></div></div>
        <Card>{mounted ? <div className="theme-options" role="radiogroup" aria-label="主题"><button className={theme === "light" ? "active" : ""} onClick={() => setTheme("light")} role="radio" aria-checked={theme === "light"}><Sun />浅色</button><button className={theme === "dark" ? "active" : ""} onClick={() => setTheme("dark")} role="radio" aria-checked={theme === "dark"}><Moon />深色</button><button className={theme === "system" ? "active" : ""} onClick={() => setTheme("system")} role="radio" aria-checked={theme === "system"}><span className="system-icon">A</span>跟随系统</button></div> : null}</Card>
      </section>
      <section className="settings-section">
        <div className="settings-heading"><Shield /><div><h2>能力画像</h2><p>由面试报告自动更新，你可以查看、修改或删除。</p></div></div>
        <Card>
          <div className="settings-fields">
            <label>优势（每行一项）<Textarea value={strengths} onChange={(event) => setStrengths(event.target.value)} /></label>
            <label>待提升（每行一项）<Textarea value={gaps} onChange={(event) => setGaps(event.target.value)} /></label>
            <label>备注<Textarea value={notes} onChange={(event) => setNotes(event.target.value)} /></label>
          </div>
          {saveProfile.error ? <Alert title={normalizeError(saveProfile.error).message} tone="danger" /> : null}
          <div className="settings-actions">
            <Button onClick={() => saveProfile.mutate()} loading={saveProfile.isPending}>保存画像</Button>
            <Button variant="ghost" onClick={() => clearProfile.mutate()} loading={clearProfile.isPending}>删除画像</Button>
          </div>
        </Card>
      </section>
      <section className="settings-section">
        <div className="settings-heading"><Shield /><div><h2>安全</h2><p>修改密码后，其他设备上的登录会被撤销。</p></div></div>
        <Card>
          <div className="settings-fields">
            <label>当前密码<Input type="password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></label>
            <label>新密码<Input type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label>
          </div>
          {savePassword.error ? <Alert title={normalizeError(savePassword.error).message} tone="danger" /> : null}
          {savePassword.isSuccess ? <Alert title="密码已更新" /> : null}
          <Button onClick={() => savePassword.mutate()} loading={savePassword.isPending} disabled={currentPassword.length < 8 || newPassword.length < 8}>更新密码</Button>
        </Card>
      </section>
      <section className="settings-section">
        <div className="settings-heading"><Download /><div><h2>隐私与数据</h2><p>简历、面试配置和回答会发送给模型供应商处理。详见<Link href="/privacy">隐私说明</Link>。</p></div></div>
        <Card>
          {exportMe.error ? <Alert title={normalizeError(exportMe.error).message} tone="danger" /> : null}
          <Button onClick={() => exportMe.mutate()} loading={exportMe.isPending}><Download />导出我的数据</Button>
          <div className="settings-fields" style={{ marginTop: 16 }}>
            <label>输入当前密码以删除账户<Input type="password" value={deletePassword} onChange={(event) => setDeletePassword(event.target.value)} /></label>
          </div>
          {deleteMe.error ? <Alert title={normalizeError(deleteMe.error).message} tone="danger" /> : null}
          <Button variant="danger" onClick={() => deleteMe.mutate()} loading={deleteMe.isPending} disabled={deletePassword.length < 8}><Trash2 />删除账户</Button>
        </Card>
      </section>
      <section className="settings-section">
        <div className="settings-heading"><LogOut /><div><h2>账户</h2><p>退出会撤销当前刷新会话，并清理本设备的访问令牌和草稿。</p></div></div>
        <Card className="logout-card"><div><h3>退出当前账户</h3><p>其他设备在修改密码前仍可能保持登录。</p></div><Button variant="danger" onClick={() => logout()}><LogOut />退出登录</Button></Card>
      </section>
    </div>
  );
}
