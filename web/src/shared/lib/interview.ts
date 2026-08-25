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

export function isFollowUpTurn(turn: { turn_kind?: string }) {
  return turn.turn_kind === "follow_up";
}

export function visibleInterviewTurns<T extends { question?: string; ordinal: number }>(
  turns: T[],
  currentOrdinal: number,
) {
  return turns.filter((turn) => Boolean(turn.question) && turn.ordinal <= currentOrdinal);
}

export function shouldCollapseChatText(text: string, collapseAfter = 360) {
  return text.length > collapseAfter || text.split("\n").length > 8;
}

export function isLiveInterviewRoomPath(pathname: string) {
  return /^\/interviews\/[^/]+$/.test(pathname) && pathname !== "/interviews/new" && pathname !== "/interviews/records";
}

export function clearInterviewDrafts() {
  if (typeof window === "undefined") return;
  const keys = Array.from({ length: localStorage.length }, (_, index) => localStorage.key(index))
    .filter((key): key is string => Boolean(key?.startsWith("im_draft:")));
  keys.forEach((key) => localStorage.removeItem(key));
}
