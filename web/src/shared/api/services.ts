import type {
  AuthResponse,
  CreateJobDescriptionRequest,
  CreateInterviewRequest,
  CreateQuestionSetRequest,
  CreateResumeUploadRequest,
  CreateResumeUploadResponse,
  InterviewReportResponse,
  InterviewSessionResponse,
  JobDescriptionPageResponse,
  JobDescriptionResponse,
  QuestionSetDetailResponse,
  ResumePageResponse,
  ResumeDetailResponse,
  TaskAcceptedResponse,
  TaskResponse,
  UserResponse,
} from "./generated/InterviewMasterComponents";
import { apiRequest } from "./client";

export const api = {
  login: (body: { email: string; password: string }) =>
    apiRequest<AuthResponse>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(body), skipRefresh: true }),
  register: (body: { email: string; password: string; display_name: string }) =>
    apiRequest<AuthResponse>("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(body),
      skipRefresh: true,
    }),
  me: () => apiRequest<UserResponse>("/api/v1/me"),
  resumes: async () => (await apiRequest<ResumePageResponse>("/api/v1/resumes")).items,
  resume: (id: string) => apiRequest<ResumeDetailResponse>(`/api/v1/resumes/${id}`),
  createResumeUpload: (body: CreateResumeUploadRequest) =>
    apiRequest<CreateResumeUploadResponse>("/api/v1/resumes/uploads", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  completeResumeUpload: (resumeId: string, versionId: string) =>
    apiRequest<TaskAcceptedResponse>(
      `/api/v1/resumes/${resumeId}/versions/${versionId}/complete`,
      { method: "POST" },
    ),
  task: (id: string) => apiRequest<TaskResponse>(`/api/v1/tasks/${id}`),
  jobs: async () => (await apiRequest<JobDescriptionPageResponse>("/api/v1/job-descriptions")).items,
  createJob: (body: CreateJobDescriptionRequest) =>
    apiRequest<JobDescriptionResponse>("/api/v1/job-descriptions", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  createQuestionSet: (body: CreateQuestionSetRequest) =>
    apiRequest<QuestionSetDetailResponse>("/api/v1/question-sets", {
      method: "POST",
      body: JSON.stringify(body),
      timeoutMs: 60_000,
    }),
  createInterview: (body: CreateInterviewRequest) =>
    apiRequest<InterviewSessionResponse>("/api/v1/interviews", {
      method: "POST",
      body: JSON.stringify(body),
      timeoutMs: 60_000,
    }),
  interview: (id: string) => apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}`),
  answerInterview: (id: string, ordinal: number, answer: string) =>
    apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/turns/${ordinal}/answer`, {
      method: "PUT",
      body: JSON.stringify({ answer }),
      timeoutMs: 60_000,
    }),
  report: (id: string) =>
    apiRequest<InterviewReportResponse>(`/api/v1/interviews/${id}/report`, {
      timeoutMs: 60_000,
    }),
};
