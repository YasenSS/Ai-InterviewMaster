import type { Metadata } from "next";
import { Suspense } from "react";

import { AuthForm } from "@/features/auth/AuthForm";

export const metadata: Metadata = { title: "注册" };

export default function Page() {
  return <Suspense><AuthForm mode="register" /></Suspense>;
}
