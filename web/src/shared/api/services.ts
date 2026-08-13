import type {
  AuthResponse,
  CreateJobDescriptionRequest,
  CreateInterviewRequest,
  CreateQuestionSetRequest,
  CreateResumeUploadRequest,
  CreateResumeUploadResponse,
  InterviewPageResponse,
  InterviewReportResponse,
  InterviewSessionResponse,
  JobDescriptionPageResponse,
  JobDescriptionResponse,
  QuestionSetDetailResponse,
  QuestionSetPageResponse,
  ResumePageResponse,
  ResumeDetailResponse,
  TaskAcceptedResponse,
  TaskPageResponse,
  TaskResponse,
  UserResponse,
  SkillProfileResponse,
  AccountExportResponse,
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
  updateMe: (body: { display_name: string }) =>
    apiRequest<UserResponse>("/api/v1/me", { method: "PATCH", body: JSON.stringify(body) }),
  logout: () => apiRequest<void>("/api/v1/auth/logout", { method: "POST" }),
  changePassword: (body: { current_password: string; new_password: string }) =>
    apiRequest<void>("/api/v1/auth/change-password", { method: "POST", body: JSON.stringify(body) }),
  exportMe: () => apiRequest<AccountExportResponse>("/api/v1/me/export"),
  deleteMe: (password: string) =>
    apiRequest<void>("/api/v1/me/delete", { method: "POST", body: JSON.stringify({ password }) }),
  skillProfile: () => apiRequest<SkillProfileResponse>("/api/v1/me/skill-profile"),
  updateSkillProfile: (body: { strengths?: string[]; gaps?: string[]; notes?: string }) =>
    apiRequest<SkillProfileResponse>("/api/v1/me/skill-profile", { method: "PATCH", body: JSON.stringify(body) }),
  deleteSkillProfile: () => apiRequest<void>("/api/v1/me/skill-profile", { method: "DELETE" }),
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
  tasks: () => apiRequest<TaskPageResponse>("/api/v1/tasks?page=1&page_size=50"),
  retryTask: (id: string) =>
    apiRequest<TaskAcceptedResponse>(`/api/v1/tasks/${id}/retry`, { method: "POST" }),
  jobs: async () => (await apiRequest<JobDescriptionPageResponse>("/api/v1/job-descriptions")).items,
  createJob: (body: CreateJobDescriptionRequest) =>
    apiRequest<JobDescriptionResponse>("/api/v1/job-descriptions", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  questionSets: async () => (await apiRequest<QuestionSetPageResponse>("/api/v1/question-sets")).items,
  questionSet: (id: string) => apiRequest<QuestionSetDetailResponse>(`/api/v1/question-sets/${id}`),
  createQuestionSet: (body: CreateQuestionSetRequest) =>
    apiRequest<TaskAcceptedResponse>("/api/v1/question-sets", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  interviews: async () => (await apiRequest<InterviewPageResponse>("/api/v1/interviews")).items,
  createInterview: (body: CreateInterviewRequest) =>
    apiRequest<InterviewSessionResponse>("/api/v1/interviews", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  interview: (id: string) => apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}`),
  answerInterview: (id: string, ordinal: number, answer: string) =>
    apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/turns/${ordinal}/answer`, {
      method: "PUT",
      body: JSON.stringify({ answer }),
    }),
  completeInterview: (id: string, confirmIncomplete = false) =>
    apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/complete`, {
      method: "POST",
      body: JSON.stringify({ confirm_incomplete: confirmIncomplete }),
    }),
  report: (id: string) => apiRequest<InterviewReportResponse>(`/api/v1/interviews/${id}/report`),
};
