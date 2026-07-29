import webapi from "./gocliRequest"
import * as components from "./InterviewMasterComponents"
export * from "./InterviewMasterComponents"

/**
 * @description "Authenticate a user and start a refresh session"
 * @param req
 */
export function login(req: components.LoginRequest) {
	return webapi.post<components.AuthResponse>(`/api/v1/auth/login`, req)
}

/**
 * @description "Revoke the current refresh session"
 */
export function logout() {
	return webapi.post<null>(`/api/v1/auth/logout`)
}

/**
 * @description "Rotate the refresh session and issue a new access token"
 */
export function refresh() {
	return webapi.post<components.AuthResponse>(`/api/v1/auth/refresh`)
}

/**
 * @description "Register a user and start a refresh session"
 * @param req
 */
export function register(req: components.RegisterRequest) {
	return webapi.post<components.AuthResponse>(`/api/v1/auth/register`, req)
}

/**
 * @description "Liveness probe"
 */
export function health() {
	return webapi.get<components.HealthResponse>(`/api/v1/health`)
}

/**
 * @description "Dependency readiness probe"
 */
export function ready() {
	return webapi.get<components.ReadinessResponse>(`/api/v1/ready`)
}

/**
 * @description "Change the authenticated user password"
 * @param req
 */
export function changePassword(req: components.ChangePasswordRequest) {
	return webapi.post<null>(`/api/v1/auth/change-password`, req)
}

/**
 * @description "Enqueue a beta ASR task"
 * @param req
 */
export function createASRTask(req: components.ASRRequest) {
	return webapi.post<components.ASRResponse>(`/api/v1/beta/asr/tasks`, req)
}

/**
 * @description "Create a browser-direct beta ASR upload"
 * @param req
 */
export function createASRUpload(req: components.ASRUploadRequest) {
	return webapi.post<components.ASRUploadResponse>(`/api/v1/beta/asr/uploads`, req)
}

/**
 * @description "Search beta company interview intelligence"
 * @param req
 */
export function searchCompanyIntel(req: components.CompanyIntelRequest) {
	return webapi.post<components.CompanyIntelResponse>(`/api/v1/beta/company-intel/search`, req)
}

/**
 * @description "Return the authenticated user's dashboard aggregates"
 */
export function dashboardSummary() {
	return webapi.get<components.DashboardSummaryResponse>(`/api/v1/dashboard/summary`)
}

/**
 * @description "Create an interview from a question snapshot"
 * @param req
 */
export function createInterview(req: components.CreateInterviewRequest) {
	return webapi.post<components.InterviewSessionResponse>(`/api/v1/interviews`, req)
}

/**
 * @description "List interviews with aggregate progress"
 * @param params
 */
export function listInterviews(params: components.InterviewListRequestParams) {
	return webapi.get<components.InterviewPageResponse>(`/api/v1/interviews`, params)
}

/**
 * @description "Return an interview with server timing state"
 * @param params
 */
export function getInterview(params: components.InterviewPathParams, id: string) {
	return webapi.get<components.InterviewSessionResponse>(`/api/v1/interviews/${id}`, params)
}

/**
 * @description "Deprecated: use PUT /interviews/:id/turns/:ordinal/answer"
 * @param params
 * @param req
 */
export function answerInterview(params: components.AnswerInterviewRequestParams, req: components.AnswerInterviewRequest, id: string) {
	return webapi.post<components.InterviewSessionResponse>(`/api/v1/interviews/${id}/answer`, params, req)
}

/**
 * @description "Complete and lock an interview"
 * @param params
 * @param req
 */
export function completeInterview(params: components.CompleteInterviewRequestParams, req: components.CompleteInterviewRequest, id: string) {
	return webapi.post<components.InterviewSessionResponse>(`/api/v1/interviews/${id}/complete`, params, req)
}

/**
 * @description "Return or generate the unique interview report"
 * @param params
 */
export function getInterviewReport(params: components.InterviewPathParams, id: string) {
	return webapi.get<components.InterviewReportResponse>(`/api/v1/interviews/${id}/report`, params)
}

/**
 * @description "Save or overwrite a specific interview answer"
 * @param params
 * @param req
 */
export function saveInterviewAnswer(params: components.SaveInterviewAnswerRequestParams, req: components.SaveInterviewAnswerRequest, id: string, ordinal: number) {
	return webapi.put<components.InterviewSessionResponse>(`/api/v1/interviews/${id}/turns/${ordinal}/answer`, params, req)
}

/**
 * @description "Skip a specific interview turn"
 * @param params
 */
export function skipInterviewTurn(params: components.InterviewTurnPathParams, id: string, ordinal: number) {
	return webapi.post<components.InterviewSessionResponse>(`/api/v1/interviews/${id}/turns/${ordinal}/skip`, params)
}

/**
 * @description "Create a job description"
 * @param req
 */
export function createJobDescription(req: components.CreateJobDescriptionRequest) {
	return webapi.post<components.JobDescriptionResponse>(`/api/v1/job-descriptions`, req)
}

/**
 * @description "List job descriptions with pagination"
 * @param params
 */
export function listJobDescriptions(params: components.PageRequestParams) {
	return webapi.get<components.JobDescriptionPageResponse>(`/api/v1/job-descriptions`, params)
}

/**
 * @description "Return a job description"
 * @param params
 */
export function getJobDescription(params: components.JobDescriptionPathParams, id: string) {
	return webapi.get<components.JobDescriptionResponse>(`/api/v1/job-descriptions/${id}`, params)
}

/**
 * @description "Update a job description and re-extract capabilities"
 * @param params
 * @param req
 */
export function updateJobDescription(params: components.UpdateJobDescriptionRequestParams, req: components.UpdateJobDescriptionRequest, id: string) {
	return webapi.patch<components.JobDescriptionResponse>(`/api/v1/job-descriptions/${id}`, params, req)
}

/**
 * @description "Delete a job description while preserving history"
 * @param params
 */
export function deleteJobDescription(params: components.JobDescriptionPathParams, id: string) {
	return webapi.delete<null>(`/api/v1/job-descriptions/${id}`, params)
}

/**
 * @description "Return the authenticated user profile"
 */
export function me() {
	return webapi.get<components.UserResponse>(`/api/v1/me`)
}

/**
 * @description "Update the authenticated user profile"
 * @param req
 */
export function updateMe(req: components.UpdateMeRequest) {
	return webapi.patch<components.UserResponse>(`/api/v1/me`, req)
}

/**
 * @description "Generate a question set"
 * @param req
 */
export function createQuestionSet(req: components.CreateQuestionSetRequest) {
	return webapi.post<components.QuestionSetDetailResponse>(`/api/v1/question-sets`, req)
}

/**
 * @description "List question sets with pagination and filtering"
 * @param params
 */
export function listQuestionSets(params: components.QuestionSetListRequestParams) {
	return webapi.get<components.QuestionSetPageResponse>(`/api/v1/question-sets`, params)
}

/**
 * @description "Return a question set and its questions"
 * @param params
 */
export function getQuestionSet(params: components.QuestionSetPathParams, id: string) {
	return webapi.get<components.QuestionSetDetailResponse>(`/api/v1/question-sets/${id}`, params)
}

/**
 * @description "Replace question-set content transactionally"
 * @param params
 * @param req
 */
export function updateQuestionSet(params: components.UpdateQuestionSetRequestParams, req: components.UpdateQuestionSetRequest, id: string) {
	return webapi.patch<components.QuestionSetDetailResponse>(`/api/v1/question-sets/${id}`, params, req)
}

/**
 * @description "Delete an unused question set"
 * @param params
 */
export function deleteQuestionSet(params: components.QuestionSetPathParams, id: string) {
	return webapi.delete<null>(`/api/v1/question-sets/${id}`, params)
}

/**
 * @description "Regenerate a question set with lineage"
 * @param params
 * @param req
 */
export function regenerateQuestionSet(params: components.RegenerateQuestionSetRequestParams, req: components.RegenerateQuestionSetRequest, id: string) {
	return webapi.post<components.QuestionSetDetailResponse>(`/api/v1/question-sets/${id}/regenerate`, params, req)
}

/**
 * @description "List resumes with pagination and filtering"
 * @param params
 */
export function listResumes(params: components.ResumeListRequestParams) {
	return webapi.get<components.ResumePageResponse>(`/api/v1/resumes`, params)
}

/**
 * @description "Return resume metadata and extracted facts"
 * @param params
 */
export function getResume(params: components.ResumePathParams, id: string) {
	return webapi.get<components.ResumeDetailResponse>(`/api/v1/resumes/${id}`, params)
}

/**
 * @description "Rename a resume"
 * @param params
 * @param req
 */
export function updateResume(params: components.UpdateResumeRequestParams, req: components.UpdateResumeRequest, id: string) {
	return webapi.patch<components.ResumeSummaryResponse>(`/api/v1/resumes/${id}`, params, req)
}

/**
 * @description "Delete an unused resume"
 * @param params
 */
export function deleteResume(params: components.ResumePathParams, id: string) {
	return webapi.delete<null>(`/api/v1/resumes/${id}`, params)
}

/**
 * @description "Reparse the current resume version"
 * @param params
 */
export function reparseResume(params: components.ResumePathParams, id: string) {
	return webapi.post<components.TaskAcceptedResponse>(`/api/v1/resumes/${id}/reparse`, params)
}

/**
 * @description "Complete a resume upload and enqueue parsing"
 * @param params
 */
export function completeResumeUpload(params: components.CompleteResumeUploadRequestParams, id: string, versionId: string) {
	return webapi.post<components.TaskAcceptedResponse>(`/api/v1/resumes/${id}/versions/${versionId}/complete`, params)
}

/**
 * @description "Create a browser-direct resume upload"
 * @param req
 */
export function createResumeUpload(req: components.CreateResumeUploadRequest) {
	return webapi.post<components.CreateResumeUploadResponse>(`/api/v1/resumes/uploads`, req)
}

/**
 * @description "List asynchronous tasks with pagination and filtering"
 * @param params
 */
export function listTasks(params: components.TaskListRequestParams) {
	return webapi.get<components.TaskPageResponse>(`/api/v1/tasks`, params)
}

/**
 * @description "Return asynchronous task state"
 * @param params
 */
export function getTask(params: components.TaskPathParams, id: string) {
	return webapi.get<components.TaskResponse>(`/api/v1/tasks/${id}`, params)
}

/**
 * @description "Retry a supported failed task"
 * @param params
 */
export function retryTask(params: components.TaskPathParams, id: string) {
	return webapi.post<components.TaskAcceptedResponse>(`/api/v1/tasks/${id}/retry`, params)
}
