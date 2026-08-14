import type { InterviewSessionResponse } from "@/shared/api/generated/InterviewMasterComponents";

type RecentData = {
  interviews: InterviewSessionResponse[];
};

export const RECENT_KEY = "im_recent_resources";
const RECENT_EVENT = "im-recent";
const empty: RecentData = { interviews: [] };

export function getRecent(): RecentData {
  if (typeof window === "undefined") return empty;
  try {
    return { ...empty, ...(JSON.parse(localStorage.getItem(RECENT_KEY) ?? "{}") as Partial<RecentData>) };
  } catch {
    return empty;
  }
}

export function rememberInterview(item: InterviewSessionResponse) {
  const data = getRecent();
  data.interviews = [item, ...data.interviews.filter((value) => value.id !== item.id)].slice(0, 20);
  localStorage.setItem(RECENT_KEY, JSON.stringify(data));
  window.dispatchEvent(new Event(RECENT_EVENT));
}

export function clearRecent() {
  localStorage.removeItem(RECENT_KEY);
  window.dispatchEvent(new Event(RECENT_EVENT));
}

export function recentSnapshot() {
  return typeof window === "undefined" ? "" : localStorage.getItem(RECENT_KEY) ?? "";
}

export function subscribeRecent(callback: () => void) {
  const storage = (event: StorageEvent) => {
    if (event.key === RECENT_KEY) callback();
  };
  window.addEventListener("storage", storage);
  window.addEventListener(RECENT_EVENT, callback);
  return () => {
    window.removeEventListener("storage", storage);
    window.removeEventListener(RECENT_EVENT, callback);
  };
}
