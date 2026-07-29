export const queryKeys = {
  me: ["me"] as const,
  resumes: (filters?: object) => ["resumes", filters ?? {}] as const,
  resume: (id: string) => ["resumes", id] as const,
  jobs: () => ["jobs"] as const,
  questionSets: () => ["question-sets"] as const,
  interview: (id: string) => ["interviews", id] as const,
  report: (id: string) => ["interviews", id, "report"] as const,
  task: (id: string) => ["tasks", id] as const,
};

export const cacheTimes = {
  reference: 5 * 60_000,
  list: 30_000,
  taskPoll: 2_000,
};
