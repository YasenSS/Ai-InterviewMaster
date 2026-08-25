"use client";

export function isMockApiEnabled() {
  return process.env.NEXT_PUBLIC_MOCK_API === "1";
}

export function MockBanner() {
  if (!isMockApiEnabled()) return null;
  return (
    <div className="mock-banner" role="status">
      Mock 环境 · 端口 3001 · 任意邮箱即可登录 · 不连接后端
    </div>
  );
}
