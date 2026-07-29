import { type ReactNode } from "react";

import { ProductShell } from "@/components/layout/ProductShell";
import { AuthGate } from "@/features/auth/AuthGate";

export default function ProductLayout({ children }: { children: ReactNode }) {
  return <AuthGate><ProductShell>{children}</ProductShell></AuthGate>;
}
