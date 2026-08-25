export const mockClock = {
  now: () => Date.now(),
};

export function iso(offsetMs = 0) {
  return new Date(mockClock.now() + offsetMs).toISOString();
}
