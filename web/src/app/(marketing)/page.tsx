import type { Metadata } from "next";

import { MarketingPage } from "@/features/auth/MarketingPage";

export const metadata: Metadata = {
  title: "AI 面试训练助手",
};

export default function Page() {
  return <MarketingPage />;
}
