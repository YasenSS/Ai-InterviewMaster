"use client";

import {
  BriefcaseBusiness,
  ClipboardList,
  FileText,
  GraduationCap,
  Home,
  Menu,
  Settings,
  Sparkles,
  UserRound,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";

import { OfflineBanner } from "@/components/feedback/States";
import { cn } from "@/shared/lib/utils";

import { useAuth } from "@/features/auth/AuthGate";

const desktopNav = [
  { href: "/dashboard", label: "仪表盘", icon: Home },
  { href: "/resumes", label: "简历", icon: FileText },
  { href: "/jobs", label: "JD", icon: BriefcaseBusiness },
  { href: "/question-sets", label: "题集", icon: ClipboardList },
  { href: "/interviews", label: "模拟面试", icon: GraduationCap },
  { href: "/tasks", label: "任务中心", icon: Sparkles },
  { href: "/settings", label: "设置", icon: Settings },
];

const mobileNav = [
  { href: "/dashboard", label: "首页", icon: Home },
  { href: "/resumes", label: "简历", icon: FileText },
  { href: "/interviews", label: "面试", icon: GraduationCap },
  { href: "/settings", label: "我的", icon: UserRound },
];

export function ProductShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { user } = useAuth();
  const [online, setOnline] = useState(true);
  const [mobileMenu, setMobileMenu] = useState(false);

  useEffect(() => {
    const sync = () => setOnline(navigator.onLine);
    sync();
    window.addEventListener("online", sync);
    window.addEventListener("offline", sync);
    return () => {
      window.removeEventListener("online", sync);
      window.removeEventListener("offline", sync);
    };
  }, []);

  const isCurrent = (href: string) => pathname === href || pathname.startsWith(`${href}/`);

  return (
    <div className="product-shell">
      {!online ? <OfflineBanner /> : null}
      <aside className="sidebar">
        <Link className="product-brand" href="/dashboard">
          <span className="logo-mark">IM</span>
          <span>InterviewMaster<small>AI 面试训练助手</small></span>
        </Link>
        <nav aria-label="产品导航">
          {desktopNav.map(({ href, label, icon: Icon }) => (
            <Link key={href} href={href} className={cn("nav-link", isCurrent(href) && "is-active")}>
              <Icon size={19} aria-hidden="true" />
              {label}
            </Link>
          ))}
        </nav>
        <div className="sidebar-user">
          <span className="avatar">{user?.display_name?.slice(0, 1) || "你"}</span>
          <span><strong>{user?.display_name ?? "用户"}</strong><small>{user?.email ?? ""}</small></span>
        </div>
      </aside>
      <header className="mobile-header">
        <Link className="product-brand" href="/dashboard"><span className="logo-mark">IM</span> InterviewMaster</Link>
        <button aria-label="打开更多导航" onClick={() => setMobileMenu((value) => !value)}><Menu /></button>
        {mobileMenu ? (
          <div className="mobile-menu">
            {desktopNav.slice(2, 6).map(({ href, label }) => <Link key={href} href={href} onClick={() => setMobileMenu(false)}>{label}</Link>)}
          </div>
        ) : null}
      </header>
      <main className="product-main">{children}</main>
      <nav className="bottom-nav" aria-label="移动端主导航">
        {mobileNav.map(({ href, label, icon: Icon }) => (
          <Link key={href} href={href} className={cn(isCurrent(href) && "is-active")}>
            <Icon size={21} aria-hidden="true" /><span>{label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
