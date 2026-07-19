export type ServiceState = "checking" | "ready" | "partial" | "offline";

export function summarizeStatus(apiReady: boolean, dependenciesReady: boolean): ServiceState {
  if (apiReady && dependenciesReady) return "ready";
  if (apiReady) return "partial";
  return "offline";
}
