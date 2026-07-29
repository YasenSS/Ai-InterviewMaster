# InterviewMaster 后端重构 TODO 与 API 契约

> 文档状态：P0/P1 已完成并通过验收
> 更新时间：2026-07-29  
> 需求基线：[`frontend-refactor-requirements.md`](./frontend-refactor-requirements.md)  
> 前端任务：[`frontend-refactor-todo.md`](./frontend-refactor-todo.md)  
> 当前契约源：`backend/api/interviewmaster.api`

## 1. 目标

本文件定义前端正式产品化所需的后端工作和 HTTP API 契约。目标是：

1. 补齐资源管理、完整面试交互、仪表盘、任务中心和正式会话接口。
2. 升级当前已有接口，使分页、错误、时间和状态表达保持一致。
3. 保证用户数据隔离、历史面试一致性和异步任务可追踪。
4. 让 `.api`、Go handler、OpenAPI 和 TypeScript SDK 始终由同一契约生成。

Beta 公司面试信息和 ASR 接口继续保留，但不进入本轮正式产品导航。

## 2. 契约与开发约定

### 2.1 唯一事实来源

接口评审通过后，必须先更新：

```text
backend/api/interviewmaster.api
```

再依次重新生成：

1. Go handler、logic 和 types。
2. `backend/api/openapi/interviewmaster.yaml`。
3. `web/src/shared/api/generated/` TypeScript SDK。

禁止在 Go、OpenAPI 或前端中长期维护与 `.api` 重复的手写 DTO。

### 2.2 状态标记

- `[ ]` 未开始。
- `[-]` 进行中。
- `[x]` 已完成并通过验收。
- `[!]` 需要产品或数据迁移决策。

### 2.3 HTTP 约定

- 正式接口统一使用 `/api/v1` 前缀。
- 除注册、登录、刷新会话、退出和健康检查外，业务接口均要求 Access Token 认证。
- Access Token 使用 `Authorization: Bearer <token>`。
- 创建成功返回 `201 Created`。
- 普通读取、更新和动作成功返回 `200 OK`。
- 异步任务已接收返回 `202 Accepted`。
- 删除和退出成功返回 `204 No Content`。
- JSON 字段统一使用 `snake_case`。
- 所有时间均使用 UTC RFC 3339，例如 `2026-07-29T08:30:00Z`。
- 所有 ID 均为 UUID 字符串。
- 空数组必须返回 `[]`，不得返回 `null`。
- 可空单值使用 `null` 或省略字段；同一字段在所有接口中保持一致。

### 2.4 分页约定

所有资源列表统一使用：

```ts
type PageResponse<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};
```

查询参数：

| 参数        | 类型    | 默认       | 规则                   |
| ----------- | ------- | ---------- | ---------------------- |
| `page`      | integer | `1`        | 最小 1                 |
| `page_size` | integer | `20`       | 1–100                  |
| `sort`      | string  | 资源默认值 | 仅接受接口声明的枚举值 |

分页查询必须提供稳定的第二排序键，例如 `updated_at DESC, id DESC`，避免翻页时重复或遗漏。

### 2.5 错误结构

所有非 2xx JSON 错误统一返回：

```ts
type ErrorResponse = {
  code: string;
  message: string;
  request_id?: string;
  details?: {
    fields?: Record<string, string[]>;
    [key: string]: unknown;
  };
};
```

示例：

```json
{
  "code": "INTERVIEW_HAS_UNANSWERED_TURNS",
  "message": "仍有未回答的问题，请确认后再完成面试",
  "request_id": "req_01J...",
  "details": {
    "ordinals": [2, 4]
  }
}
```

HTTP 状态规则：

| 状态  | 用途                                     |
| ----- | ---------------------------------------- |
| `400` | 请求格式、字段或查询参数不合法           |
| `401` | 未认证、Access Token 无效或会话失效      |
| `403` | 已认证但无权执行操作                     |
| `404` | 当前用户范围内资源不存在                 |
| `409` | 资源被引用、状态冲突、重复完成等业务冲突 |
| `413` | 上传文件过大                             |
| `415` | 不支持的文件类型                         |
| `429` | 请求频率超限                             |
| `500` | 未分类服务端错误                         |
| `503` | 数据库、Redis、对象存储或模型服务不可用  |

生产环境不得把 SQL、堆栈、对象存储路径、模型原始异常或其他敏感细节放入 `message` 或 `details`。

### 2.6 用户隔离

- 每个资源查询和变更都必须带当前 `user_id` 条件。
- 访问其他用户资源统一按 `404` 处理，避免泄露资源是否存在。
- 关联资源创建时必须同时验证所有资源属于当前用户。
- 列表、仪表盘和任务统计不得跨用户聚合。

## 3. 通用数据类型

以下 TypeScript 仅用于清晰描述 JSON 契约，最终类型以 `.api` 生成结果为准。

### 3.1 枚举

```ts
type ResumeStatus =
  "draft" | "uploading" | "pending" | "processing" | "completed" | "failed";

type TaskStatus = "pending" | "running" | "succeeded" | "failed";
type QuestionSetStatus = "ready" | "archived";
type InterviewStatus = "draft" | "active" | "completed" | "abandoned";
type InterviewTurnState = "unstarted" | "answering" | "answered" | "skipped";
```

枚举值新增时，必须同步更新 `.api`、OpenAPI、前端状态映射和测试。

### 3.2 用户与认证

```ts
type UserResponse = {
  id: string;
  email: string;
  display_name: string;
};

type AuthResponse = {
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
  user: UserResponse;
};
```

### 3.3 简历

```ts
type ResumeSummaryResponse = {
  id: string;
  title: string;
  status: ResumeStatus;
  version_id?: string;
  file_name?: string;
  created_at: string;
  updated_at: string;
};

type ResumeFactResponse = {
  type: string;
  key: string;
  value: unknown;
  excerpt: string;
  confidence: number;
};

type ResumeDetailResponse = ResumeSummaryResponse & {
  version_no?: number;
  content_type?: string;
  size_bytes?: number;
  uploaded_at?: string;
  processed_at?: string;
  parse_error?: string;
  facts: ResumeFactResponse[];
};
```

### 3.4 JD

```ts
type JobDescriptionResponse = {
  id: string;
  company?: string;
  title: string;
  content: string;
  capabilities: string[];
  created_at: string;
  updated_at: string;
};
```

### 3.5 题集

```ts
type QuestionResponse = {
  id: string;
  ordinal: number;
  question: string;
  intent: string;
  expected_points: string[];
  follow_up_hint?: string;
};

type QuestionSetSummaryResponse = {
  id: string;
  resume: {
    id: string;
    title: string;
  };
  job_description?: {
    id: string;
    company?: string;
    title: string;
  };
  target_role?: string;
  status: QuestionSetStatus;
  question_count: number;
  source_question_set_id?: string;
  created_at: string;
  updated_at: string;
};

type QuestionSetDetailResponse = QuestionSetSummaryResponse & {
  questions: QuestionResponse[];
};
```

### 3.6 面试

```ts
type InterviewTurnResponse = {
  ordinal: number;
  question: string;
  answer?: string;
  state: InterviewTurnState;
  started_at?: string;
  answered_at?: string;
  skipped_at?: string;
  time_spent_seconds: number;
};

type InterviewSummaryResponse = {
  id: string;
  title: string;
  status: InterviewStatus;
  question_set?: {
    id: string;
    target_role?: string;
  };
  resume: {
    id: string;
    title: string;
  };
  question_count: number;
  answered_count: number;
  skipped_count: number;
  current_ordinal: number;
  overall_score?: number;
  started_at?: string;
  completed_at?: string;
  duration_seconds: number;
  created_at: string;
  updated_at: string;
};

type InterviewSessionResponse = InterviewSummaryResponse & {
  job_description?: {
    id: string;
    company?: string;
    title: string;
  };
  question_duration_seconds: number;
  turns: InterviewTurnResponse[];
};
```

`time_spent_seconds` 为服务端在响应时计算或持久化的非负整数。活动题可随当前时间增长，前端在两次服务端同步之间自行显示连续计时。

### 3.7 报告

```ts
type TurnReportResponse = {
  ordinal: number;
  question: string;
  answer?: string;
  score: number;
  critique: string;
  golden_answer: string;
  evidence: string[];
};

type InterviewReportResponse = {
  id: string;
  session_id: string;
  status: "completed";
  overall_score: number;
  strengths: string[];
  improvements: string[];
  next_steps: string[];
  quality_passed: boolean;
  turns: TurnReportResponse[];
  created_at: string;
  updated_at: string;
};
```

### 3.8 异步任务

```ts
type TaskReferenceResponse = {
  type: "resume" | "resume_version" | "interview" | "other";
  id: string;
  title?: string;
};

type TaskErrorResponse = {
  code: string;
  message: string;
};

type TaskResponse = {
  id: string;
  type: string;
  status: TaskStatus;
  progress: number;
  reference: TaskReferenceResponse;
  error?: TaskErrorResponse;
  result?: unknown;
  retry_of_task_id?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
};

type TaskAcceptedResponse = {
  task_id: string;
  status: TaskStatus;
};
```

## 4. 输入校验基线

| 字段       | 规则                                          |
| ---------- | --------------------------------------------- |
| 邮箱       | 去除首尾空格、转小写、合法邮箱、最大 254 字符 |
| 密码       | 8–72 字符；不得记录明文                       |
| 显示名称   | 去除首尾空格，1–80 字符                       |
| 资源标题   | 去除首尾空格，1–120 字符                      |
| 公司名称   | 可选；去除首尾空格，最大 120 字符             |
| JD 正文    | 去除首尾空格，20–50,000 字符                  |
| 目标岗位   | 可选；去除首尾空格，最大 120 字符             |
| 面试回答   | 去除纯空白判断；1–20,000 字符                 |
| 简历文件   | PDF、DOCX、TXT；1 byte–20 MiB                 |
| 题目正文   | 1–2,000 字符                                  |
| 考察意图   | 1–1,000 字符                                  |
| 期望回答点 | 最多 20 项，每项 1–500 字符                   |
| 追问提示   | 可选；最大 1,000 字符                         |

字段错误放入 `details.fields`，例如：

```json
{
  "code": "VALIDATION_ERROR",
  "message": "提交内容有误，请检查后重试",
  "details": {
    "fields": {
      "content": ["JD 正文至少需要 20 个字符"]
    }
  }
}
```

## 5. P0：认证与用户接口

### BE-AUTH-01 注册

```http
POST /api/v1/auth/register
```

请求：

```ts
type RegisterRequest = {
  email: string;
  password: string;
  display_name: string;
};
```

响应：`201 AuthResponse`，同时设置 Refresh Cookie。

规则：

- 邮箱标准化后保持唯一。
- 密码使用项目批准的强哈希算法存储。
- 注册成功即建立登录会话。
- 邮箱已占用返回 `409 EMAIL_ALREADY_REGISTERED`。

### BE-AUTH-02 登录

```http
POST /api/v1/auth/login
```

请求：

```ts
type LoginRequest = {
  email: string;
  password: string;
};
```

响应：`200 AuthResponse`，同时设置 Refresh Cookie。

规则：

- 邮箱不存在和密码错误统一返回 `401 AUTH_INVALID_CREDENTIALS`。
- 不通过错误文案泄露账户是否存在。
- 登录接口需要按 IP 和账户维度限流。

### BE-AUTH-03 刷新会话

```http
POST /api/v1/auth/refresh
```

请求：无 JSON Body；从 HttpOnly Refresh Cookie 读取会话。

响应：`200 AuthResponse`，轮换 Refresh Cookie。

规则：

- Refresh Token 只以哈希形式持久化。
- 默认有效期 30 天，最终以服务端配置为准。
- Cookie 属性：`HttpOnly`、生产环境 `Secure`、`SameSite=Lax`、`Path=/api/v1/auth`。
- 每次刷新轮换 Token，旧 Token 立即失效。
- 已撤销、过期或重放返回 `401 AUTH_SESSION_INVALID`。

### BE-AUTH-04 退出

```http
POST /api/v1/auth/logout
```

响应：`204 No Content`。

规则：

- 撤销当前 Refresh Session。
- 清除 Refresh Cookie。
- 重复退出保持幂等。

### BE-USER-01 当前用户

```http
GET /api/v1/me
```

响应：`200 UserResponse`。

### BE-USER-02 修改资料

```http
PATCH /api/v1/me
```

请求：

```ts
type UpdateMeRequest = {
  display_name: string;
};
```

响应：`200 UserResponse`。

### BE-USER-03 修改密码

```http
POST /api/v1/auth/change-password
```

请求：

```ts
type ChangePasswordRequest = {
  current_password: string;
  new_password: string;
};
```

响应：`204 No Content`。

规则：

- 当前密码错误返回 `400 CURRENT_PASSWORD_INCORRECT`。
- 新旧密码相同返回 `400 PASSWORD_UNCHANGED`。
- 修改成功后撤销该用户除当前会话外的其他 Refresh Session。

### 认证数据 TODO

- [x] 新增 Refresh Session 表，至少包含用户、Token 哈希、过期、撤销、创建和最后使用时间。
- [x] 增加按用户和 Token 哈希查询的索引。
- [x] Access Token 使用短有效期，具体秒数通过 `expires_in` 返回。
- [x] 增加刷新、重放、撤销和多用户隔离测试。

## 6. P0：简历接口

### BE-RESUME-01 创建上传凭证

```http
POST /api/v1/resumes/uploads
```

请求：

```ts
type CreateResumeUploadRequest = {
  title: string;
  file_name: string;
  content_type:
    | "application/pdf"
    | "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    | "text/plain"
    | "application/octet-stream";
  size_bytes: number;
};
```

响应：

```ts
type CreateResumeUploadResponse = {
  resume_id: string;
  version_id: string;
  upload_url: string;
  upload_headers: Record<string, string>;
  expires_at: string;
};
```

状态：`201 Created`。

规则：

- 服务端同时校验扩展名、声明 MIME 和大小；`application/octet-stream` 只可作为合法扩展名缺少浏览器 MIME 时的兼容值。
- 上传完成后必须检查实际文件类型，不能只信任扩展名和客户端声明。
- 对象 Key 必须位于当前用户命名空间。
- 预签名 URL 默认 15 分钟有效，最终以 `expires_at` 为准。
- 不返回对象存储内部凭证。

错误：

- `413 RESUME_FILE_TOO_LARGE`
- `415 RESUME_FILE_TYPE_UNSUPPORTED`

### BE-RESUME-02 完成上传并启动解析

```http
POST /api/v1/resumes/:id/versions/:versionId/complete
```

响应：`202 TaskAcceptedResponse`。

规则：

- 验证 Resume 和 Version 均属于当前用户且相互关联。
- 验证对象真实存在，大小和声明值一致。
- 将简历状态置为 `pending` 并创建 `resume.parse` 任务。
- 同一版本已有 `pending/running/succeeded` 任务时返回原任务，保持幂等。
- 任务成功后简历为 `completed`；失败后简历为 `failed` 并保存安全错误摘要。

### BE-RESUME-03 简历列表

```http
GET /api/v1/resumes
```

查询参数：

| 参数        | 可选值                              |
| ----------- | ----------------------------------- |
| `status`    | ResumeStatus，可重复或逗号分隔      |
| `page`      | 正整数                              |
| `page_size` | 1–100                               |
| `sort`      | `updated_at_desc`、`updated_at_asc` |

响应：`200 PageResponse<ResumeSummaryResponse>`。

> 当前接口直接返回数组，必须升级为统一分页结构。

### BE-RESUME-04 简历详情

```http
GET /api/v1/resumes/:id
```

响应：`200 ResumeDetailResponse`。

规则：

- Facts 按 `type`、创建顺序稳定排序。
- `parse_error` 只返回清洗后的用户可理解摘要。
- 未解析或无事实时返回 `facts: []`。

### BE-RESUME-05 重命名

```http
PATCH /api/v1/resumes/:id
```

请求：

```ts
type UpdateResumeRequest = {
  title: string;
};
```

响应：`200 ResumeSummaryResponse`。

### BE-RESUME-06 删除

```http
DELETE /api/v1/resumes/:id
```

响应：`204 No Content`。

规则：

- 删除前检查关联题集和面试。
- 存在关联时返回 `409 RESUME_IN_USE`，并在 `details` 中返回引用数量。
- 不允许依赖数据库级联静默删除历史题集、面试或报告。
- 无关联时同步删除数据库记录；对象文件删除失败进入可重试清理机制，不阻塞已完成的数据库删除。
- 资源已经不存在时仍返回 `204`，保持删除幂等。

冲突示例：

```json
{
  "code": "RESUME_IN_USE",
  "message": "这份简历已用于题集或面试，暂时不能删除",
  "details": {
    "question_set_count": 2,
    "interview_count": 1
  }
}
```

### BE-RESUME-07 重新解析

```http
POST /api/v1/resumes/:id/reparse
```

响应：`202 TaskAcceptedResponse`。

规则：

- 仅当前版本文件存在时允许。
- 已有 pending/running 解析任务时返回原任务。
- 新任务开始前不删除当前 facts；成功后事务性替换，失败时保留旧 facts 并把状态标为 `failed`。

### 简历 TODO

- [x] 列表升级分页和筛选。
- [x] 详情补版本号、类型、大小、处理时间、错误和置信度。
- [x] 实现重命名、关联检查删除和重新解析。
- [x] 完成接口幂等处理。
- [x] 增加对象存储一致性和多用户隔离测试。

## 7. P0：JD 接口

### BE-JOB-01 创建

```http
POST /api/v1/job-descriptions
```

请求：

```ts
type CreateJobDescriptionRequest = {
  company?: string;
  title: string;
  content: string;
};
```

响应：`201 JobDescriptionResponse`。

规则：

- 创建时提取能力标签。
- 能力提取失败不得伪造标签；可返回空数组并记录可追踪错误，或整体返回明确错误。
- 相同能力标签去重并保持稳定顺序。

### BE-JOB-02 列表

```http
GET /api/v1/job-descriptions
```

查询参数：

| 参数        | 可选值                              |
| ----------- | ----------------------------------- |
| `page`      | 正整数                              |
| `page_size` | 1–100                               |
| `sort`      | `updated_at_desc`、`updated_at_asc` |

响应：`200 PageResponse<JobDescriptionResponse>`。

> 当前接口直接返回数组，必须升级为统一分页结构。

### BE-JOB-03 详情

```http
GET /api/v1/job-descriptions/:id
```

响应：`200 JobDescriptionResponse`。

### BE-JOB-04 更新

```http
PATCH /api/v1/job-descriptions/:id
```

请求：

```ts
type UpdateJobDescriptionRequest = {
  company?: string;
  title?: string;
  content?: string;
};
```

响应：`200 JobDescriptionResponse`。

规则：

- 至少提供一个字段。
- `title` 或 `content` 变化时重新提取能力标签。
- 更新和标签替换在同一事务中完成。

### BE-JOB-05 删除

```http
DELETE /api/v1/job-descriptions/:id
```

响应：`204 No Content`。

规则：

- 题集和面试保留已生成内容，关联 `job_description_id` 置空。
- 删除不回写或重新生成已有题目、回答和报告。
- 资源已经不存在时仍返回 `204`，保持删除幂等。

### JD TODO

- [x] 列表升级分页。
- [x] 实现详情、更新和删除。
- [x] 增加内容更新后标签重提取测试。
- [x] 验证删除后历史题集和面试仍可读取。

## 8. P0：题集接口

### BE-QSET-01 生成题集

```http
POST /api/v1/question-sets
```

请求：

```ts
type CreateQuestionSetRequest = {
  resume_id: string;
  job_description_id?: string;
  target_role?: string;
};
```

响应：`201 QuestionSetDetailResponse`。

规则：

- 简历必选且状态必须为 `completed`。
- JD 可选；提供时必须属于当前用户。
- JD 为空时允许目标岗位为空，但服务端记录该事实，不伪造岗位。
- 生成的题号必须从 1 开始连续递增。
- 生成失败不得写入半成品题集。

错误：

- `409 RESUME_NOT_PARSED`
- `404 RESUME_NOT_FOUND`
- `404 JOB_DESCRIPTION_NOT_FOUND`
- `503 QUESTION_GENERATION_UNAVAILABLE`

### BE-QSET-02 列表

```http
GET /api/v1/question-sets
```

查询参数：

| 参数        | 可选值                              |
| ----------- | ----------------------------------- |
| `status`    | `ready`、`archived`                 |
| `resume_id` | UUID                                |
| `page`      | 正整数                              |
| `page_size` | 1–100                               |
| `sort`      | `created_at_desc`、`created_at_asc` |

响应：`200 PageResponse<QuestionSetSummaryResponse>`。

### BE-QSET-03 详情

```http
GET /api/v1/question-sets/:id
```

响应：`200 QuestionSetDetailResponse`。

### BE-QSET-04 更新题集

```http
PATCH /api/v1/question-sets/:id
```

请求：

```ts
type QuestionInput = {
  ordinal: number;
  question: string;
  intent: string;
  expected_points: string[];
  follow_up_hint?: string;
};

type UpdateQuestionSetRequest = {
  target_role?: string;
  status?: QuestionSetStatus;
  questions?: QuestionInput[];
};
```

响应：`200 QuestionSetDetailResponse`。

规则：

- 至少提供一个字段。
- 提供 `questions` 时表示完整替换题目集合，不是局部追加。
- 题号必须为 `1..N`、唯一且连续。
- 题目集合至少 1 题、最多 50 题。
- 更新在单事务中完成。
- 已创建面试保存了题目快照，因此编辑题集不修改历史面试题目。

### BE-QSET-05 删除

```http
DELETE /api/v1/question-sets/:id
```

响应：`204 No Content`。

规则：

- 存在关联面试时返回 `409 QUESTION_SET_IN_USE`，建议用户归档而不是删除。
- 无关联时删除题集和其 questions。
- 不得级联删除面试。
- 资源已经不存在时仍返回 `204`，保持删除幂等。

### BE-QSET-06 重新生成

```http
POST /api/v1/question-sets/:id/regenerate
```

请求：

```ts
type RegenerateQuestionSetRequest = {
  job_description_id?: string;
  target_role?: string;
};
```

响应：`201 QuestionSetDetailResponse`。

规则：

- 始终创建新题集，不覆盖原题集。
- 未提供覆盖字段时继承原题集的简历、JD 和目标岗位。
- 新记录保存 `source_question_set_id`。
- 原题集及其历史面试保持不变。

### 题集数据与 TODO

- [x] `question_sets` 增加 `updated_at`。
- [x] `question_sets` 增加可空 `source_question_set_id` 自引用外键。
- [x] 实现列表、详情、完整替换更新、关联检查删除和重新生成。
- [x] 增加题号、事务回滚、历史面试快照和多用户隔离测试。

## 9. P0：面试接口

### 9.1 面试计时和状态规则

创建面试时：

- `status = active`。
- `started_at = now()`。
- `question_duration_seconds` 默认 `180`。
- 第一题 `started_at = now()`，后续题为未开始。

题目状态由持久化字段推导：

| 条件                                  | 状态        |
| ------------------------------------- | ----------- |
| `started_at` 为空                     | `unstarted` |
| 已开始，`answer` 和 `skipped_at` 为空 | `answering` |
| `answer` 非空                         | `answered`  |
| `answer` 为空且 `skipped_at` 非空     | `skipped`   |

规则：

- 面试计时不可暂停。
- 单题超时不自动提交、不自动跳过。
- 回答被覆盖时更新 `answered_at`，并清除 `skipped_at`。
- 首次到达下一题时写入其 `started_at`。
- `completed` 后所有题目写操作返回冲突。
- 服务端是计时和完成状态的唯一权威来源。

### BE-INTERVIEW-01 创建面试

```http
POST /api/v1/interviews
```

请求：

```ts
type CreateInterviewRequest = {
  resume_id: string;
  question_set_id: string;
  job_description_id?: string;
  title: string;
  question_duration_seconds?: number;
};
```

响应：`201 InterviewSessionResponse`。

规则：

- `resume_id` 必须与题集关联简历一致。
- 可选 JD 必须属于当前用户。
- `question_duration_seconds` 第一阶段只允许 `180`；保留字段用于后续配置。
- 在同一事务中创建 Session 和题目快照 Turns。
- 题集无题目时返回 `409 QUESTION_SET_EMPTY`。

### BE-INTERVIEW-02 面试列表

```http
GET /api/v1/interviews
```

查询参数：

| 参数        | 可选值                                                 |
| ----------- | ------------------------------------------------------ |
| `status`    | `draft`、`active`、`completed`、`abandoned`            |
| `page`      | 正整数                                                 |
| `page_size` | 1–100                                                  |
| `sort`      | `updated_at_desc`、`updated_at_asc`、`created_at_desc` |

响应：`200 PageResponse<InterviewSummaryResponse>`。

列表需要一次查询或受控批量查询返回题目数、已答数、跳过数和可选报告总分，禁止逐条 N+1 查询。

### BE-INTERVIEW-03 面试详情

```http
GET /api/v1/interviews/:id
```

响应：`200 InterviewSessionResponse`。

规则：

- 返回足以在刷新后恢复双计时器的所有字段。
- `duration_seconds`：
  - 活动中：`now - started_at`。
  - 已完成：`completed_at - started_at`。
- Turns 按 `ordinal` 升序。

### BE-INTERVIEW-04 保存或覆盖指定题回答

```http
PUT /api/v1/interviews/:id/turns/:ordinal/answer
```

请求：

```ts
type SaveInterviewAnswerRequest = {
  answer: string;
};
```

响应：`200 InterviewSessionResponse`。

规则：

- 仅 `active` 面试可写。
- 允许首次回答和覆盖回答。
- 纯空白回答视为校验错误，不等同于跳过。
- 保存后设置 `answered_at = now()`，清除 `skipped_at`。
- 题目此前未开始时同时设置 `started_at = now()`。
- 操作按最终请求内容幂等。
- 若操作使顺序流程到达下一未开始题，服务端启动该题并更新 `current_ordinal`。
- 使用事务和行锁避免并发请求产生错误状态。

错误：

- `404 INTERVIEW_NOT_FOUND`
- `404 INTERVIEW_TURN_NOT_FOUND`
- `409 INTERVIEW_ALREADY_COMPLETED`

### BE-INTERVIEW-05 跳过指定题

```http
POST /api/v1/interviews/:id/turns/:ordinal/skip
```

请求：无 Body。

响应：`200 InterviewSessionResponse`。

规则：

- 仅 `active` 面试可操作。
- 无答案时记录 `skipped_at`；已有答案时返回 `409 TURN_ALREADY_ANSWERED`，前端可选择保留答案或先覆盖。
- 题目此前未开始时同时设置 `started_at = now()`。
- 重复跳过同一题保持幂等，不重复改变时间。
- 启动下一未开始题并更新 `current_ordinal`。

### BE-INTERVIEW-06 完成整场面试

```http
POST /api/v1/interviews/:id/complete
```

请求：

```ts
type CompleteInterviewRequest = {
  confirm_incomplete: boolean;
};
```

响应：`200 InterviewSessionResponse`。

规则：

- 存在 `unstarted` 或 `skipped` 题目且 `confirm_incomplete=false` 时返回 `409 INTERVIEW_HAS_UNANSWERED_TURNS`。
- 冲突 `details.ordinals` 返回所有未回答题号。
- 确认后将面试设为 `completed`、写入 `completed_at` 和 `duration_seconds`。
- 完成后所有回答、跳过和重复状态变更被锁定。
- 重复完成返回当前已完成 Session，保持幂等。
- 完成动作不必同步生成报告；首次获取报告时生成并复用。

### BE-INTERVIEW-07 旧顺序回答接口迁移

```http
POST /api/v1/interviews/:id/answer
```

- [x] 新接口上线期间保留旧接口，标记 deprecated。
- [x] 旧接口内部复用“保存当前题回答”的新业务逻辑。
- [!] 前端已切换到指定题接口；完成一个兼容发布周期后，再安排删除旧接口。
- [!] 删除前必须确认无调用方和自动化测试依赖。

### 面试数据迁移

`interview_sessions` 增加：

```text
started_at                  timestamptz
duration_seconds            integer NOT NULL DEFAULT 0
question_duration_seconds   integer NOT NULL DEFAULT 180
```

`interview_turns` 增加：

```text
started_at          timestamptz
skipped_at          timestamptz
time_spent_seconds  integer NOT NULL DEFAULT 0
```

约束：

- 所有 duration/time_spent 字段不得小于 0。
- `question_duration_seconds` 第一阶段限制为 180。
- 现有 `active` 会话的数据迁移以 `created_at` 作为缺失 `started_at` 的回填值。
- 现有 `completed` 会话使用 `completed_at - created_at` 回填时长。
- 已回答题使用 `created_at`/`answered_at` 回填题目开始和用时；无法准确恢复时记录迁移说明，不伪造精确含义。

### 面试 TODO

- [x] 完成数据库迁移和回填。
- [x] 实现列表、指定题保存、跳过和完成接口。
- [x] 升级详情响应字段。
- [x] 所有状态变更使用事务和必要行锁。
- [x] 增加刷新恢复、并发保存、重复请求、完成锁定和多用户隔离测试。

## 10. P0：报告接口

### BE-REPORT-01 获取或生成报告

```http
GET /api/v1/interviews/:id/report
```

响应：`200 InterviewReportResponse`。

规则：

- 仅 `completed` 面试可以生成报告。
- 报告不存在时生成并持久化。
- 报告已存在时直接复用，禁止每次查询重复生成。
- 并发首次请求只能成功写入一份报告。
- 逐题报告必须返回原问题和用户回答。
- 未回答题允许 `answer` 为空，但仍需给出与缺答相符的评分和点评。
- `score` 和 `overall_score` 范围为 0–100。
- 报告模型或依赖服务失败时返回可重试错误，不写入半成品报告。

错误：

- `404 INTERVIEW_NOT_FOUND`
- `409 INTERVIEW_NOT_COMPLETED`
- `503 REPORT_GENERATION_UNAVAILABLE`

### 报告 TODO

- [x] 响应补 `status`、问题、原回答、创建和更新时间。
- [x] 保持现有一场面试唯一一份报告约束。
- [x] 增加并发生成复用、未回答题和质量门禁测试。
- [x] 第一阶段不开发服务端 PDF；由浏览器打印导出。

## 11. P1：任务中心接口

### BE-TASK-01 单任务

```http
GET /api/v1/tasks/:id
```

响应：`200 TaskResponse`。

升级要求：

- 在现有字段上补关联对象、结构化错误、重试来源和起止时间。
- `progress` 必须在 0–100。
- 失败任务不得把 Worker 原始异常直接返回前端。

### BE-TASK-02 任务列表

```http
GET /api/v1/tasks
```

查询参数：

| 参数        | 可选值                                      |
| ----------- | ------------------------------------------- |
| `status`    | `pending`、`running`、`succeeded`、`failed` |
| `type`      | 任务类型，如 `resume.parse`                 |
| `page`      | 正整数                                      |
| `page_size` | 1–100                                       |
| `sort`      | `created_at_desc`、`created_at_asc`         |

响应：`200 PageResponse<TaskResponse>`。

正式产品第一阶段主要返回 `resume.parse`。Beta ASR 任务仍受用户隔离保护，但前端正式任务中心是否展示由前端产品配置决定。

### BE-TASK-03 重试失败任务

```http
POST /api/v1/tasks/:id/retry
```

响应：`202 TaskAcceptedResponse`。

规则：

- 只有 `failed` 且任务类型支持重试时允许。
- 创建新 Task ID，并记录 `retry_of_task_id`。
- 原任务保持只读，不改回 pending。
- 已存在针对同一失败任务的 pending/running 重试时返回该任务，保持幂等。
- 不支持重试返回 `409 TASK_NOT_RETRYABLE`。

### 任务数据与 TODO

- [x] `async_tasks` 增加可空 `retry_of_task_id` 自引用外键。
- [x] 设计结构化、安全的 `error_code` 与用户错误摘要字段。
- [x] 实现列表、筛选、分页和重试。
- [x] Worker 对失败、成功和完成时间进行一致更新。
- [x] 增加重复重试和跨用户访问测试。

## 12. P1：仪表盘接口

### BE-DASH-01 仪表盘摘要

```http
GET /api/v1/dashboard/summary
```

响应：

```ts
type DashboardSummaryResponse = {
  counts: {
    resumes: number;
    job_descriptions: number;
    question_sets: number;
    interviews: number;
    completed_interviews: number;
  };
  average_score?: number;
  score_trend: Array<{
    date: string; // YYYY-MM-DD，按 UTC 聚合
    average_score: number;
    interview_count: number;
  }>;
  improvement_topics: Array<{
    label: string;
    count: number;
  }>;
  recent_resumes: ResumeSummaryResponse[];
  recent_job_descriptions: JobDescriptionResponse[];
  recent_interviews: InterviewSummaryResponse[];
  active_tasks: TaskResponse[];
};
```

规则：

- `average_score` 在没有已完成报告时省略，不返回虚构的 0。
- 趋势默认最近 30 天，只统计已有报告的已完成面试。
- 每类最近资源最多 5 条。
- 活跃任务只包含 `pending/running`，最多 5 条。
- 改进主题来自真实报告数据，最多 5 条；无数据返回 `[]`。
- 查询需避免 N+1，并为用户、状态和时间条件建立必要索引。

### 仪表盘 TODO

- [x] 实现摘要聚合查询。
- [x] 明确改进主题的规范化策略，避免同义文本被无限拆分。
- [x] 增加新用户全空、部分数据和完整数据测试。
- [x] 对聚合查询进行执行计划和性能检查。

## 13. 数据库迁移总表

| 优先级 | 表                    | 变更                                                          |
| ------ | --------------------- | ------------------------------------------------------------- |
| P0     | 新 Refresh Session 表 | 正式刷新和退出会话                                            |
| P0     | `question_sets`       | `updated_at`、`source_question_set_id`                        |
| P0     | `interview_sessions`  | `started_at`、`duration_seconds`、`question_duration_seconds` |
| P0     | `interview_turns`     | `started_at`、`skipped_at`、`time_spent_seconds`              |
| P1     | `async_tasks`         | `retry_of_task_id`、结构化错误字段                            |
| P1     | 报告相关数据          | 如需稳定聚合，增加规范化改进主题或派生表                      |

要求：

- [x] 每项变更提供 Goose Up/Down migration。
- [x] Up migration 对现有数据可执行并有明确回填规则。
- [x] 新索引考虑 `user_id + status + time` 的列表查询。
- [x] 不修改既有 migration 文件，使用新编号增量迁移。
- [x] Migration、sqlc 查询和应用逻辑在同一变更中提交。

## 14. 接口实现优先级

### P0-A：阻塞前端基础闭环

- [x] 统一错误响应和校验错误。
- [x] 简历/JD 列表分页升级。
- [x] 简历详情字段升级。
- [x] 题集列表和详情。
- [x] 面试详情计时字段。
- [x] 报告补问题和原回答。

### P0-B：阻塞正式资源管理

- [x] 简历重命名、删除、重新解析。
- [x] JD 详情、更新、删除。
- [x] 题集更新、删除、重新生成。

### P0-C：阻塞完整面试体验

- [x] 面试列表。
- [x] 保存/覆盖指定题回答。
- [x] 跳过指定题。
- [x] 主动完成整场面试。
- [x] 服务端计时和刷新恢复。

### P1：产品数据闭环

- [x] 仪表盘摘要。
- [x] 任务列表和重试。
- [x] 修改资料和密码。
- [x] Refresh Cookie 和退出。

### P2：本轮不实施

- 找回和重置密码。
- 邮箱验证。
- 注销账户。
- 简历多版本管理 UI 所需接口。
- 报告服务端 PDF。
- 公司面试信息和 ASR 正式产品化。

## 15. 兼容与发布策略

### 15.1 列表响应升级

当前简历和 JD 列表直接返回数组，正式契约改为 `PageResponse<T>`。项目尚未正式发布，因此优先直接升级 `/api/v1` 并同步生成前端 SDK；如果已有外部调用方，则必须：

1. 盘点调用方。
2. 临时提供兼容适配或新版本路径。
3. 在调用方迁移完成后移除旧响应。

### 15.2 面试回答迁移

新前端统一使用：

```http
PUT /interviews/:id/turns/:ordinal/answer
```

旧 `POST /interviews/:id/answer` 在一个发布周期内保留并标记 deprecated。旧接口和新接口必须复用同一领域逻辑，避免行为分叉。

### 15.3 SDK 生成

每次接口变更的 CI 必须检查：

- `.api` 可以成功生成 Go、OpenAPI 和 TypeScript。
- 生成文件与仓库内容一致，无未提交差异。
- 前端 `typecheck` 通过。
- 后端和前端契约测试通过。

## 16. 测试与验收

### 16.1 单元和集成测试

- [x] 每个接口的正常、校验失败、未认证、资源不存在和状态冲突。
- [x] 所有资源的跨用户隔离。
- [x] 列表分页边界、稳定排序和空列表。
- [x] 删除关联冲突和历史数据保留。
- [x] 上传完成、重复完成和对象不存在。
- [x] 解析失败、重新解析和重复重试。
- [x] 题集完整替换的事务回滚。
- [x] 面试跳题、返回改答、重复完成和完成后锁定。
- [x] 面试刷新后的时间字段一致。
- [x] 报告首次生成、重复复用和并发复用。
- [x] Refresh Token 轮换、重放和退出撤销。

### 16.2 契约测试

- [x] HTTP 状态、响应字段和枚举与 `.api` 一致。
- [x] ErrorResponse 包含稳定 `code` 和请求 ID。
- [x] 所有时间为合法 UTC RFC 3339。
- [x] 所有空集合返回 `[]`。
- [x] SDK 可直接完成前端核心流程，不需要手写补充 DTO。

### 16.3 性能与可靠性

- [x] 列表和仪表盘查询无 N+1。
- [x] 默认分页下核心列表查询达到项目约定延迟目标。
- [x] 删除、完成面试、重试任务等动作幂等或返回明确冲突。
- [x] Worker 状态最终落库，失败不会永久停留在 `running`。
- [x] 日志包含 request ID、用户 ID、资源 ID和错误码，但不包含密码、Token 和完整简历正文。

## 17. 最终后端验收清单

- [x] 前端所需 P0/P1 接口均写入 `.api`。
- [x] Go、OpenAPI 和 TypeScript SDK 已重新生成。
- [x] “简历 → JD → 题集 → 面试 → 报告”完整流程可通过 API 自动化测试。
- [x] 面试支持跳过、返回覆盖、主动完成和刷新计时恢复。
- [x] 完成后的面试无法再修改。
- [x] 简历、JD 和题集管理具备明确关联规则。
- [x] 仪表盘和任务中心只返回当前用户真实数据。
- [x] 认证支持短期 Access Token、Refresh Cookie 和退出撤销。
- [x] 统一分页、错误、时间和状态约定全部落地。
- [x] 所有新增 migration 可在现有数据上成功执行。
- [x] 后端测试、生成检查和项目验收脚本全部通过。
