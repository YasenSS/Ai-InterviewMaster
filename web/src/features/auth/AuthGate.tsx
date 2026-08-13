"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useContext, useEffect, type ReactNode } from "react";

import { Skeleton } from "@/components/ui/Display";
import { api } from "@/shared/api/services";
import { authStorage } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import type { UserResponse } from "@/shared/api/generated/InterviewMasterComponents";
import { clearRecent } from "@/shared/lib/recent";
import { clearInterviewDrafts } from "@/shared/lib/interview";
import { useHydrated } from "@/shared/hooks/useRecent";

type AuthContextValue = {
  user?: UserResponse;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue>({ logout: async () => undefined });

export function AuthGate({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const queryClient = useQueryClient();
  const hydrated = useHydrated();
  const token = hydrated ? authStorage.get() : null;
  const userQuery = useQuery({
    queryKey: queryKeys.me,
    queryFn: api.me,
    enabled: Boolean(token),
    retry: false,
  });

  useEffect(() => {
    if (hydrated && !token) {
      router.replace(`/login?return_to=${encodeURIComponent(pathname)}`);
    }
  }, [hydrated, pathname, router, token]);

  if (!token || userQuery.isPending) {
    return (
      <div className="app-loading" aria-label="正在验证登录状态">
        <Skeleton className="skeleton-block" />
        <Skeleton className="skeleton-line" />
      </div>
    );
  }

  const logout = async () => {
    try {
      await api.logout();
    } catch {
      // Client cleanup still proceeds if the refresh cookie is already gone.
    }
    authStorage.clear();
    clearInterviewDrafts();
    clearRecent();
    queryClient.clear();
    router.replace("/login");
  };

  return <AuthContext.Provider value={{ user: userQuery.data, logout }}>{children}</AuthContext.Provider>;
}

export const useAuth = () => useContext(AuthContext);
