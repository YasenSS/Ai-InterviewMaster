"use client";

import { useMemo, useSyncExternalStore } from "react";

import { getRecent, recentSnapshot, subscribeRecent } from "@/shared/lib/recent";

export function useRecent() {
  const snapshot = useSyncExternalStore(subscribeRecent, recentSnapshot, () => "");
  return useMemo(() => {
    if (!snapshot) return { questionSets: [], interviews: [], taskIds: [] };
    return getRecent();
  }, [snapshot]);
}

export function useHydrated() {
  return useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );
}
