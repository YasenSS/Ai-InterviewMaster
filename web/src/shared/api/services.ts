import type {
  AuthResponse,
  CreateResumeUploadRequest,
  CreateResumeUploadResponse,
  InterviewPageResponse,
  InterviewReportResponse,
  InterviewSessionResponse,
  ResumePageResponse,
  ResumeDetailResponse,
  UserResponse,
  SkillProfileResponse,
  AccountExportResponse,
} from "./generated/InterviewMasterComponents";
import { apiRequest } from "./client";

export type CreateInterviewInput = {
  resume_id: string;
  primary_language: string;
  target_company: string;
  question_duration_seconds?: number;
};

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
	apiRequest<ResumeDetailResponse>(
      `/api/v1/resumes/${resumeId}/versions/${versionId}/complete`,
      { method: "POST" },
    ),
  interviews: async () => (await apiRequest<InterviewPageResponse>("/api/v1/interviews")).items,
  interviewPage: (params: { page: number; page_size: number; status?: string }) => {
    const search = new URLSearchParams({ page: String(params.page), page_size: String(params.page_size) });
    if (params.status) search.append("status", params.status);
    return apiRequest<InterviewPageResponse>(`/api/v1/interviews?${search.toString()}`);
  },
  createInterview: (body: CreateInterviewInput) =>
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
	 skipInterview: (id: string, ordinal: number) =>
	   apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/turns/${ordinal}/skip`, {
	     method: "POST",
	   }),
	 retryInterviewPreparation: (id: string) =>
	   apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/preparation/retry`, { method: "POST" }),
	 retryInterviewDecision: (id: string) =>
	   apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/next-turn/retry`, { method: "POST" }),
	 fallbackInterviewDecision: (id: string) =>
	   apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/next-turn/fallback`, { method: "POST" }),
  completeInterview: (id: string, confirmIncomplete = false) =>
    apiRequest<InterviewSessionResponse>(`/api/v1/interviews/${id}/complete`, {
      method: "POST",
      body: JSON.stringify({ confirm_incomplete: confirmIncomplete }),
    }),
  report: (id: string) => apiRequest<InterviewReportResponse>(`/api/v1/interviews/${id}/report`),
	 retryInterviewReport: (id: string) =>
	   apiRequest<InterviewReportResponse>(`/api/v1/interviews/${id}/report/retry`, { method: "POST" }),
};
