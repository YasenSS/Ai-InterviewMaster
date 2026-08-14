"use client";

import {
  FileText,
  GraduationCap,
  History,
  Home,
  Menu,
  PlayCircle,
  Settings,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";

import { OfflineBanner } from "@/components/feedback/States";
import { useAuth } from "@/features/auth/AuthGate";
import { cn } from "@/shared/lib/utils";

const primaryNav = [
  { href: "/dashboard", label: "仪表盘", icon: Home },
  { href: "/resumes", label: "简历", icon: FileText },
];

const interviewNav = [
  { href: "/interviews/new", label: "开始面试", icon: PlayCircle },
  { href: "/interviews/records", label: "面试记录", icon: History },
];

const utilityNav = [{ href: "/settings", label: "设置", icon: Settings }];

const mobileNav = [
  { href: "/dashboard", label: "首页", icon: Home },
  { href: "/resumes", label: "简历", icon: FileText },
  { href: "/interviews/new", label: "面试", icon: GraduationCap },
  { href: "/interviews/records", label: "记录", icon: History },
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

  const isCurrent = (href: string) => {
    if (href === "/interviews/records" && /^\/interviews\/[^/]+\/(record|report)$/.test(pathname)) return true;
    return pathname === href || pathname.startsWith(`${href}/`);
  };
  const renderLinks = (items: typeof primaryNav, child = false) =>
    items.map(({ href, label, icon: Icon }) => (
      <Link
        key={href}
        href={href}
        className={cn("nav-link", child && "nav-sub-link", isCurrent(href) && "is-active")}
      >
        <Icon size={19} aria-hidden="true" />
        {label}
      </Link>
    ));

  return (
    <div className="product-shell">
      {!online ? <OfflineBanner /> : null}
      <aside className="sidebar">
        <Link className="product-brand" href="/dashboard">
          <span className="logo-mark">IM</span>
          <span>
            InterviewMaster<small>AI 面试训练助手</small>
          </span>
        </Link>
        <nav aria-label="产品导航">
          {renderLinks(primaryNav)}
          <span className="nav-section-label">模拟面试</span>
          {renderLinks(interviewNav, true)}
          {renderLinks(utilityNav)}
        </nav>
        <div className="sidebar-user">
          <span className="avatar">{user?.display_name?.slice(0, 1) || "你"}</span>
          <span>
            <strong>{user?.display_name ?? "用户"}</strong>
            <small>{user?.email ?? ""}</small>
          </span>
        </div>
      </aside>
      <header className="mobile-header">
        <Link className="product-brand" href="/dashboard">
          <span className="logo-mark">IM</span> InterviewMaster
        </Link>
        <button aria-label="打开更多导航" onClick={() => setMobileMenu((value) => !value)}>
          <Menu />
        </button>
        {mobileMenu ? (
          <div className="mobile-menu">
            <Link href="/settings" onClick={() => setMobileMenu(false)}>设置</Link>
          </div>
        ) : null}
      </header>
      <main className="product-main">{children}</main>
      <nav className="bottom-nav" aria-label="移动端主导航">
        {mobileNav.map(({ href, label, icon: Icon }) => (
          <Link key={href} href={href} className={cn(isCurrent(href) && "is-active")}>
            <Icon size={21} aria-hidden="true" />
            <span>{label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
