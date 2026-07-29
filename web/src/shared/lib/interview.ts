export const QUESTION_SECONDS = 180;

export function elapsedSeconds(startedAt: string, now = Date.now()) {
  const start = new Date(startedAt).getTime();
  if (Number.isNaN(start)) return 0;
  return Math.max(0, Math.floor((now - start) / 1000));
}

export function questionRemaining(enteredAt: number, now = Date.now()) {
  return Math.max(0, QUESTION_SECONDS - Math.floor((now - enteredAt) / 1000));
}

export function draftKey(userId: string, interviewId: string, ordinal: number) {
  return `im_draft:${userId}:${interviewId}:${ordinal}`;
}

export function clearInterviewDrafts() {
  if (typeof window === "undefined") return;
  const keys = Array.from({ length: localStorage.length }, (_, index) => localStorage.key(index))
    .filter((key): key is string => Boolean(key?.startsWith("im_draft:")));
  keys.forEach((key) => localStorage.removeItem(key));
}
