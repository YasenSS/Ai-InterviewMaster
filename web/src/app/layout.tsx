import type { Metadata } from "next";
import type { ReactNode } from "react";

import "./globals.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: {
    default: "InterviewMaster｜AI 面试训练助手",
    template: "%s｜InterviewMaster",
  },
  description: "基于你的简历和目标岗位，完成可复盘的 AI 模拟面试。",
  openGraph: {
    title: "InterviewMaster｜AI 面试训练助手",
    description: "基于你的简历和目标岗位，完成可复盘的 AI 模拟面试。",
    locale: "zh_CN",
    type: "website",
  },
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body><Providers>{children}</Providers></body>
    </html>
  );
}
