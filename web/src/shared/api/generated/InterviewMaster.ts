import webapi from "./gocliRequest"
import * as components from "./InterviewMasterComponents"
export * from "./InterviewMasterComponents"

/**
 * @description 
 * @param req
 */
export function login(req: components.LoginRequest) {
	return webapi.post<components.AuthResponse>(`/api/v1/auth/login`, req)
}

/**
 * @description 
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
 * @description 
 * @param req
 */
export function createASRTask(req: components.ASRRequest) {
	return webapi.post<components.ASRResponse>(`/api/v1/beta/asr/tasks`, req)
}

/**
 * @description 
 * @param req
 */
export function createASRUpload(req: components.ASRUploadRequest) {
	return webapi.post<components.ASRUploadResponse>(`/api/v1/beta/asr/uploads`, req)
}

/**
 * @description 
 * @param req
 */
export function searchCompanyIntel(req: components.CompanyIntelRequest) {
	return webapi.post<components.CompanyIntelResponse>(`/api/v1/beta/company-intel/search`, req)
}

/**
 * @description 
 * @param req
 */
export function createInterview(req: components.CreateInterviewRequest) {
	return webapi.post<components.InterviewSessionResponse>(`/api/v1/interviews`, req)
}

/**
 * @description 
 * @param params
 */
export function getInterview(params: components.InterviewPathParams, id: string) {
	return webapi.get<components.InterviewSessionResponse>(`/api/v1/interviews/${id}`, params)
}

/**
 * @description 
 * @param params
 * @param req
 */
export function answerInterview(params: components.AnswerInterviewRequestParams, req: components.AnswerInterviewRequest, id: string) {
	return webapi.post<components.InterviewSessionResponse>(`/api/v1/interviews/${id}/answer`, params, req)
}

/**
 * @description 
 * @param params
 */
export function getInterviewReport(params: components.InterviewPathParams, id: string) {
	return webapi.get<components.InterviewReportResponse>(`/api/v1/interviews/${id}/report`, params)
}

/**
 * @description 
 * @param req
 */
export function createJobDescription(req: components.JobDescriptionRequest) {
	return webapi.post<components.JobDescriptionResponse>(`/api/v1/job-descriptions`, req)
}

/**
 * @description 
 */
export function listJobDescriptions() {
	return webapi.get<Array<components.JobDescriptionResponse>>(`/api/v1/job-descriptions`)
}

/**
 * @description 
 */
export function me() {
	return webapi.get<components.UserResponse>(`/api/v1/me`)
}

/**
 * @description 
 * @param req
 */
export function createQuestionSet(req: components.CreateQuestionSetRequest) {
	return webapi.post<components.QuestionSetResponse>(`/api/v1/question-sets`, req)
}

/**
 * @description 
 */
export function listResumes() {
	return webapi.get<Array<components.ResumeResponse>>(`/api/v1/resumes`)
}

/**
 * @description 
 * @param params
 */
export function getResume(params: components.ResumePathParams, id: string) {
	return webapi.get<components.ResumeDetailResponse>(`/api/v1/resumes/${id}`, params)
}

/**
 * @description 
 * @param params
 */
export function completeResumeUpload(params: components.CompleteResumeUploadRequestParams, id: string, versionId: string) {
	return webapi.post<components.CompleteResumeUploadResponse>(`/api/v1/resumes/${id}/versions/${versionId}/complete`, params)
}

/**
 * @description 
 * @param req
 */
export function createResumeUpload(req: components.CreateResumeUploadRequest) {
	return webapi.post<components.CreateResumeUploadResponse>(`/api/v1/resumes/uploads`, req)
}

/**
 * @description 
 * @param params
 */
export function getTask(params: components.TaskPathParams, id: string) {
	return webapi.get<components.TaskResponse>(`/api/v1/tasks/${id}`, params)
}
