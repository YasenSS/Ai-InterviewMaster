# InterviewMaster 前端重构 TODO

> 文档状态：待开发  
> 更新时间：2026-07-29  
> 需求基线：[`frontend-refactor-requirements.md`](./frontend-refactor-requirements.md)  
> 后端契约：[`backend-refactor-todo.md`](./backend-refactor-todo.md)  
> 前端目录：`web/`

## 1. 目标与边界

本轮目标是将当前单页技术验证工作台重构为面向真实用户的完整 Web 产品，完成以下核心闭环：

```text
官网 → 注册/登录 → 上传并解析简历 → 添加可选 JD → 生成题集
     → 创建模拟面试 → 逐题作答 → 完成面试 → 查看报告
```

本轮正式产品不包含：

- 公司面试信息 Beta。
- ASR 语音转写 Beta。
- 实时语音、视频或数字人面试。
- 英文界面、社区、职位投递和招聘方后台。
- 找回密码、邮箱验证和注销账户。
- 简历与多个 JD 的复杂长期绑定。

禁止为后端尚未实现的能力制作假成功交互。对应操作必须隐藏或明确禁用，并关联后端 TODO。

## 2. 开发约定

### 2.1 状态标记

- `[ ]` 未开始。
- `[-]` 进行中。
- `[x]` 已完成并通过验收。
- `[!]` 被后端接口或产品决策阻塞。

### 2.2 完成定义

单项任务只有同时满足以下条件才能标记为完成：

1. 桌面端和移动端均可使用。
2. 浅色、深色和跟随系统模式正常。
3. 加载、空、成功、失败和无权限状态完整。
4. 键盘操作、焦点状态和表单可访问名称正确。
5. 不使用虚构业务数据，不直接暴露后端原始错误。
6. 相关组件测试或端到端测试通过。
7. `lint`、`typecheck`、单元测试和生产构建通过。

## 3. 路由与页面清单

| ID          | 路由                      | 页面         | 核心要求                                          | 依赖接口                     |
| ----------- | ------------------------- | ------------ | ------------------------------------------------- | ---------------------------- |
| FE-ROUTE-01 | `/`                       | 产品官网     | 中文价值主张、三步流程、能力介绍、场景、CTA、页脚 | 无                           |
| FE-ROUTE-02 | `/login`                  | 登录         | 邮箱/密码、即时校验、错误反馈、返回原页面         | `POST /auth/login`           |
| FE-ROUTE-03 | `/register`               | 注册         | 显示名称、邮箱、密码、确认密码，成功后自动登录    | `POST /auth/register`        |
| FE-ROUTE-04 | `/dashboard`              | 仪表盘       | 新训练、统计、趋势、最近资源、活跃任务、空状态    | `GET /dashboard/summary`     |
| FE-ROUTE-05 | `/resumes`                | 简历列表     | 状态/时间筛选、重命名、删除、重新解析             | 简历列表及管理接口           |
| FE-ROUTE-06 | `/resumes/new`            | 上传简历     | 文件校验、预签名上传、真实进度、解析任务跟踪      | 简历上传及任务接口           |
| FE-ROUTE-07 | `/resumes/[id]`           | 简历详情     | 文件信息、版本、结构化事实、来源摘录、管理操作    | 简历详情及管理接口           |
| FE-ROUTE-08 | `/jobs`                   | JD 列表      | 公司、岗位、标签、时间、查看/编辑/删除            | JD 列表及管理接口            |
| FE-ROUTE-09 | `/jobs/new`               | 新建 JD      | 公司、岗位、长文本、字符计数和校验                | `POST /job-descriptions`     |
| FE-ROUTE-10 | `/jobs/[id]`              | JD 详情/编辑 | 查看能力标签、编辑、删除                          | JD 详情及管理接口            |
| FE-ROUTE-11 | `/question-sets`          | 题集列表     | 筛选、摘要、查看、删除、重新生成                  | 题集列表及管理接口           |
| FE-ROUTE-12 | `/question-sets/new`      | 生成题集     | 简历必选、JD/岗位可选、材料摘要、生成状态         | `POST /question-sets`        |
| FE-ROUTE-13 | `/question-sets/[id]`     | 题集详情     | 题目预览、意图、回答点、追问、编辑、开始面试      | 题集详情及管理接口           |
| FE-ROUTE-14 | `/interviews`             | 面试历史     | 进行中/已完成、进度、分数、筛选、继续/查看        | `GET /interviews`            |
| FE-ROUTE-15 | `/interviews/new`         | 创建面试     | 选择题集、自动摘要、标题、计时说明                | 题集列表、创建面试           |
| FE-ROUTE-16 | `/interviews/[id]`        | 面试房间     | 单题、双计时、跳过、返回、改答、提交、恢复        | 面试详情及控制接口           |
| FE-ROUTE-17 | `/interviews/[id]/report` | 面试报告     | 总分、图表、逐题分析、打印/PDF                    | `GET /interviews/:id/report` |
| FE-ROUTE-18 | `/tasks`                  | 任务中心     | 类型、对象、状态、进度、筛选、失败重试            | 任务列表及重试接口           |
| FE-ROUTE-19 | `/settings`               | 设置         | 资料、主题、修改密码、退出                        | 用户及认证接口               |

## 4. P0：工程基础

### FE-BASE-01 初始化依赖与目录

- [ ] 接入 Tailwind CSS。
- [ ] 接入 shadcn/ui，并只保留实际使用的组件。
- [ ] 接入 TanStack Query。
- [ ] 接入 React Hook Form 和 Zod。
- [ ] 接入 `next-themes`。
- [ ] 接入 Testing Library 和 Playwright。
- [ ] 建立以下目录：

```text
web/src/
  app/
    (marketing)/
    (auth)/
    (product)/
  components/
    ui/
    layout/
    feedback/
  features/
    auth/
    dashboard/
    resumes/
    jobs/
    question-sets/
    interviews/
    reports/
    tasks/
    settings/
  shared/
    api/
    config/
    hooks/
    lib/
    types/
  styles/
```

验收：

- 页面文件只做路由和模块编排，不包含大段 API、表单或业务状态逻辑。
- 通用无业务组件位于 `components/`，业务组件位于对应 `features/`。
- 官网构建产物不加载登录后业务模块。

### FE-BASE-02 统一 API Client

- [ ] 以 `web/src/shared/api/generated/` 的 goctl 生成代码为基础。
- [ ] 在生成 SDK 外建立业务 API Client，禁止页面直接调用 `fetch`。
- [ ] 自动注入 `Authorization: Bearer <access_token>`。
- [ ] 统一解析后端 `ErrorResponse`。
- [ ] 支持 `AbortSignal`、请求超时和上传进度。
- [ ] 保存并展示响应中的 `request_id`，便于问题追踪。
- [ ] 统一处理 401：尝试刷新会话，失败后跳转登录。
- [ ] 登录跳转携带安全的站内 `return_to`，重新登录后返回原页面。
- [ ] 统一定义 Query Key、缓存时间、失效规则和轮询策略。
- [ ] 不在页面或 feature 中重复手写服务端响应类型。

API 错误归一化后的前端结构：

```ts
type AppError = {
  code: string;
  message: string;
  requestId?: string;
  fieldErrors?: Record<string, string[]>;
  details?: unknown;
};
```

### FE-BASE-03 Query 与表单规范

- [ ] API 数据、缓存、轮询和请求状态全部由 TanStack Query 管理。
- [ ] 表单使用 React Hook Form。
- [ ] 输入校验使用 Zod，错误信息为简体中文。
- [ ] 页面短期 UI 状态使用组件状态或 reducer。
- [ ] 第一阶段不引入 Redux/Zustand。
- [ ] 面试权威状态来自服务端，本地只保存未提交草稿和当前展示题号。

### FE-BASE-04 鉴权与路由保护

- [ ] 未登录用户只能访问官网、登录和注册。
- [ ] 登录用户访问 `/login` 或 `/register` 时跳转 `/dashboard`。
- [ ] 登录成功默认进入 `/dashboard`，存在 `return_to` 时优先返回原页面。
- [ ] 服务端和客户端均不得短暂渲染无权限的产品内容。
- [ ] Access Token 仅用于短期请求认证；后端会话升级完成后使用 HttpOnly Refresh Cookie。
- [ ] 退出后清理客户端缓存、草稿和用户相关内存状态。

## 5. P0：设计系统与产品壳层

### FE-UI-01 主题与 Design Tokens

- [ ] 使用 CSS Variables 定义背景、表面、文字、边框、品牌、成功、警告和危险色。
- [ ] 支持浅色、深色、跟随系统。
- [ ] 默认跟随系统并持久化用户选择。
- [ ] SSR/首屏不发生主题闪烁。
- [ ] 组件中不散落需求文档规定的硬编码颜色。
- [ ] 尊重 `prefers-reduced-motion`。

### FE-UI-02 基础组件

- [ ] Button、IconButton、Input、Textarea、Select、Checkbox。
- [ ] FormField、FieldError、PasswordInput、CharacterCounter。
- [ ] Card、Badge、Progress、Skeleton、Tooltip。
- [ ] Alert、Toast、EmptyState、ErrorState、OfflineBanner。
- [ ] Dialog、ConfirmDialog、Drawer、DropdownMenu。
- [ ] Pagination、FilterBar、ResponsiveList。
- [ ] 组件具有清晰 focus ring、禁用态、加载态和可访问名称。

### FE-UI-03 响应式产品布局

- [ ] 桌面端左侧导航：仪表盘、简历、JD、题集、模拟面试、任务中心、设置。
- [ ] 移动端顶部栏加底部导航：首页、简历、面试、我的。
- [ ] JD、题集、任务中心从首页快捷入口或“我的”进入。
- [ ] 最小宽度 320px，无横向滚动。
- [ ] 触控目标不小于 44×44px。
- [ ] 固定底部操作区适配安全区域。
- [ ] 移动端表格转换为卡片或摘要列表。
- [ ] 移动端弹窗转换为抽屉或全屏页。

## 6. P0：官网与认证

### FE-AUTH-01 产品官网

- [ ] 顶部品牌、功能、使用流程、登录和“免费开始”入口。
- [ ] 首屏文案：“基于你的简历和目标岗位，完成可复盘的 AI 模拟面试。”
- [ ] 三步流程：准备材料、开始面试、获得报告。
- [ ] 四项核心能力：简历理解、定制题集、逐题面试、复盘报告。
- [ ] 应届生和社招用户场景。
- [ ] 底部 CTA 和基础页脚。
- [ ] 移动端首屏无需长距离滚动即可看到价值主张和主 CTA。
- [ ] 配置中文 Metadata、Open Graph 基础信息和合理的 SEO 语义结构。

### FE-AUTH-02 登录

- [ ] 邮箱、密码、密码可见性切换。
- [ ] 邮箱格式和必填即时校验。
- [ ] 按钮内提交状态，防止重复提交。
- [ ] 凭证错误使用统一用户文案，不泄露账户是否存在。
- [ ] 不显示不可用的第三方登录和找回密码入口。

依赖：

- `POST /api/v1/auth/login`

### FE-AUTH-03 注册

- [ ] 显示名称、邮箱、密码、确认密码。
- [ ] 密码规则与后端一致。
- [ ] 确认密码只在前端校验，不传给后端。
- [ ] 注册成功后自动建立会话并进入仪表盘。
- [ ] 邮箱已占用时定位到邮箱字段。

依赖：

- `POST /api/v1/auth/register`

## 7. P0：简历

### FE-RESUME-01 简历列表

- [ ] 显示标题、文件名、更新时间和解析状态。
- [ ] 支持状态筛选、更新时间排序和分页。
- [ ] 桌面端列表/表格，移动端纵向卡片。
- [ ] 新用户空状态说明简历用途并提供上传主操作。
- [ ] `pending`、`processing` 状态显示任务进度入口。
- [ ] `failed` 状态显示可理解原因和重新解析入口。
- [ ] 重命名使用轻量表单。
- [ ] 删除前显示关联影响并二次确认。

依赖：

- `GET /api/v1/resumes`
- `PATCH /api/v1/resumes/:id`
- `DELETE /api/v1/resumes/:id`
- `POST /api/v1/resumes/:id/reparse`

### FE-RESUME-02 上传与解析

- [ ] 只允许 PDF、DOCX、TXT。
- [ ] 文件大小必须大于 0 且不超过 20 MiB。
- [ ] 请求上传凭证后，将文件直接 `PUT` 到预签名 URL。
- [ ] 使用后端返回的 `upload_headers`，显示真实上传百分比。
- [ ] 上传完成后通知后端启动解析。
- [ ] 根据返回 `task_id` 轮询任务状态。
- [ ] 轮询间隔：处理中 2 秒；页面不可见时暂停主动轮询。
- [ ] 成功后失效简历列表缓存并跳转详情。
- [ ] 失败后展示摘要、请求 ID和重新解析入口。
- [ ] 上传中离开页面时提示用户上传会中断。

依赖：

- `POST /api/v1/resumes/uploads`
- 对预签名 URL 执行 `PUT`
- `POST /api/v1/resumes/:id/versions/:versionId/complete`
- `GET /api/v1/tasks/:id`

### FE-RESUME-03 简历详情

- [ ] 展示标题、原始文件名、版本号、类型、大小、上传时间和解析状态。
- [ ] 按个人信息、教育、工作、项目、技能等类别展示 facts。
- [ ] 每条事实展示来源摘录；置信度仅作为辅助信息，不伪装为准确率结论。
- [ ] 支持重命名、重新解析和删除。
- [ ] 解析中展示任务状态或刷新入口。
- [ ] 解析失败展示安全处理后的失败原因。

依赖：

- `GET /api/v1/resumes/:id`
- `PATCH /api/v1/resumes/:id`
- `DELETE /api/v1/resumes/:id`
- `POST /api/v1/resumes/:id/reparse`

## 8. P0：JD

### FE-JOB-01 JD 列表

- [ ] 显示公司、岗位、能力标签和更新时间。
- [ ] 支持分页、时间排序、查看、编辑和删除。
- [ ] 公司为空时不显示空占位列。
- [ ] 移动端使用卡片布局。
- [ ] 空状态引导创建 JD，同时说明 JD 为可选材料。

### FE-JOB-02 新建与编辑

- [ ] 字段：公司（可选）、岗位名称、JD 正文。
- [ ] 正文适合粘贴长文本并显示字符数。
- [ ] 前后端采用相同长度限制。
- [ ] 保存后跳转详情并展示提取出的能力标签。
- [ ] 内容或岗位变化时提示能力标签会重新提取。
- [ ] 删除前说明已生成题目和历史面试不会被回写修改。

依赖：

- `POST /api/v1/job-descriptions`
- `GET /api/v1/job-descriptions`
- `GET /api/v1/job-descriptions/:id`
- `PATCH /api/v1/job-descriptions/:id`
- `DELETE /api/v1/job-descriptions/:id`

## 9. P0：题集

### FE-QSET-01 题集生成

- [ ] 简历必选，且只有 `completed` 的简历可选。
- [ ] JD 可选。
- [ ] 目标岗位可选；未选择 JD 时提示用户填写。
- [ ] 提交前展示简历、JD 和目标岗位摘要。
- [ ] 生成期间显示明确状态和可离开页面说明。
- [ ] 防止重复提交。
- [ ] 成功后失效题集列表并跳转题集详情。
- [ ] 失败时保留用户选择并允许重试。

### FE-QSET-02 题集列表与详情

- [ ] 列表展示目标岗位、关联简历、可选 JD、题目数量、状态和创建时间。
- [ ] 详情按顺序展示问题、考察意图、期望回答点和追问提示。
- [ ] 支持编辑题目文本、意图、回答点、追问和顺序。
- [ ] 保存完整题目集合时进行本地结构校验。
- [ ] 支持删除和重新生成。
- [ ] 重新生成明确创建新题集，原题集和历史面试保持不变。
- [ ] “开始模拟面试”为详情页唯一主操作。

依赖：

- `POST /api/v1/question-sets`
- `GET /api/v1/question-sets`
- `GET /api/v1/question-sets/:id`
- `PATCH /api/v1/question-sets/:id`
- `DELETE /api/v1/question-sets/:id`
- `POST /api/v1/question-sets/:id/regenerate`

## 10. P0：面试

### FE-INTERVIEW-01 面试历史与创建

- [ ] 历史页区分进行中和已完成。
- [ ] 显示标题、关联题集、创建时间、回答进度、状态和可选总分。
- [ ] 支持状态筛选、时间排序和分页。
- [ ] 进行中可继续，已完成只能查看和进入报告。
- [ ] 创建页选择题集并自动展示简历、JD、题目数量。
- [ ] 可修改面试标题。
- [ ] 开始前说明每题默认 3 分钟、整场参考时长和不可暂停规则。
- [ ] 创建成功后立即进入面试房间。

### FE-INTERVIEW-02 面试房间

- [ ] 专注模式，一次只展示一道题。
- [ ] 顶部显示退出入口、当前题剩余时间、整场已用时间和总进度。
- [ ] 主区显示题目和必要回答提示。
- [ ] 多行文本输入；输入期间本地持久化草稿。
- [ ] 底部操作：上一题、跳过、下一题/保存回答、完成面试。
- [ ] 桌面侧栏或移动抽屉展示题目导航和状态。
- [ ] 状态文案：未开始、作答中、已回答、已跳过。
- [ ] 移动端键盘弹出后仍可看到主要提交操作。

计时规则：

- [ ] 当前题默认倒计时 180 秒。
- [ ] 整场参考时长为题目数 × 180 秒。
- [ ] 当前题剩余 30 秒时同时使用图标/文字和颜色提醒。
- [ ] 单题超时只显示超时状态，不自动提交或跳题。
- [ ] 整场开始后不可暂停。
- [ ] 刷新后根据服务端 `started_at` 恢复，不在客户端重置。
- [ ] 计时计算覆盖系统时钟变化和页面后台恢复。

作答规则：

- [ ] 保存回答调用指定题目接口，可覆盖旧回答。
- [ ] 跳过后进入下一题，整场完成前可以返回补答。
- [ ] 修改已回答题目后覆盖旧答案。
- [ ] 已完成面试不显示可编辑输入控件。
- [ ] 存在未回答或跳过题时，完成操作二次确认。
- [ ] 后端返回未完成题冲突时，定位并标出对应题号。

离开与断网：

- [ ] 未保存文本在站内跳转和关闭页面时提示。
- [ ] 已保存答案与计时依靠服务端恢复。
- [ ] 断网时显示全局提醒并保留本地草稿。
- [ ] 网络恢复后不自动覆盖服务端答案，由用户确认保存。
- [ ] 草稿键至少包含用户 ID、面试 ID 和题号。
- [ ] 成功保存、完成面试或退出登录时清理对应草稿。

依赖：

- `GET /api/v1/interviews`
- `POST /api/v1/interviews`
- `GET /api/v1/interviews/:id`
- `PUT /api/v1/interviews/:id/turns/:ordinal/answer`
- `POST /api/v1/interviews/:id/turns/:ordinal/skip`
- `POST /api/v1/interviews/:id/complete`

兼容说明：

- 旧接口 `POST /api/v1/interviews/:id/answer` 只支持顺序首次作答。
- 新面试房间完成迁移后不得继续依赖旧接口。

## 11. P0：报告

### FE-REPORT-01 报告展示

- [ ] 展示总分、完成状态和报告生成时间。
- [ ] 使用可访问的横向条形图展示各题分数。
- [ ] 展示优势、优先改进方向和下一步训练建议。
- [ ] 逐题展示原问题、用户回答、得分、点评、参考答案和证据。
- [ ] 图表除颜色外同时使用数字和文本表达。
- [ ] 报告不存在时发起获取/生成；失败后允许重试。
- [ ] 面试未完成时给出明确说明和返回面试入口。
- [ ] 移动端为单列布局。

### FE-REPORT-02 打印与 PDF

- [ ] 提供浏览器打印操作。
- [ ] 打印样式隐藏导航、按钮、Toast 等非报告内容。
- [ ] 打印分页避免标题与正文、问题与分析被不合理拆开。
- [ ] 第一阶段通过浏览器“另存为 PDF”导出，不依赖服务端 PDF。

依赖：

- `GET /api/v1/interviews/:id/report`

## 12. P1：仪表盘、任务和设置

### FE-DASH-01 仪表盘

- [ ] “开始新训练”为主操作。
- [ ] 展示最近简历、JD、面试和进行中面试。
- [ ] 展示训练次数、可空平均分和近期趋势。
- [ ] 展示高频改进方向。
- [ ] 展示正在执行或失败的任务。
- [ ] 数据不足时展示真实空状态，不填充虚构数字。
- [ ] 新用户展示从上传简历开始的首次使用引导。

### FE-TASK-01 任务中心

- [ ] 展示任务类型、关联对象、状态、进度和起止时间。
- [ ] 状态：等待中、处理中、成功、失败。
- [ ] 支持状态、任务类型筛选和分页。
- [ ] 失败任务显示安全错误摘要和重试入口。
- [ ] 第一阶段正式导航只展示简历解析任务。
- [ ] 结构预留其他任务类型，但不展示 Beta ASR 入口。

### FE-SETTINGS-01 设置

- [ ] 个人资料：显示名称和只读邮箱。
- [ ] 外观：浅色、深色、跟随系统。
- [ ] 安全：当前密码、新密码、确认新密码。
- [ ] 账户：退出登录。
- [ ] 不显示未实现的注销账户入口。

依赖：

- `GET /api/v1/dashboard/summary`
- `GET /api/v1/tasks`
- `POST /api/v1/tasks/:id/retry`
- `GET /api/v1/me`
- `PATCH /api/v1/me`
- `POST /api/v1/auth/change-password`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`

## 13. API 使用规则

### 13.1 时间与枚举

- 服务端时间均为 UTC RFC 3339 字符串。
- 前端按用户本地时区展示，提交时不自行拼接无时区时间。
- 前端使用生成的字符串联合类型处理状态，并为未知状态提供安全兜底。

关键状态：

```ts
type ResumeStatus =
  "draft" | "uploading" | "pending" | "processing" | "completed" | "failed";

type TaskStatus = "pending" | "running" | "succeeded" | "failed";
type QuestionSetStatus = "ready" | "archived";
type InterviewStatus = "draft" | "active" | "completed" | "abandoned";
type InterviewTurnState = "unstarted" | "answering" | "answered" | "skipped";
```

### 13.2 分页

所有列表接口统一消费：

```ts
type PageResponse<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};
```

规则：

- 页码从 1 开始。
- `page_size` 默认 20，最大 100。
- 空列表返回 `items: []`，不返回 `null`。
- 筛选和排序进入 URL Search Params，以便刷新和分享后恢复。

### 13.3 错误处理

后端错误格式：

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

前端行为：

| HTTP 状态 | 前端行为                           |
| --------- | ---------------------------------- |
| 400       | 显示字段或请求级校验错误           |
| 401       | 尝试刷新会话；失败后跳转登录       |
| 403       | 显示无权限，不暴露其他用户资源信息 |
| 404       | 显示资源不存在状态                 |
| 409       | 显示业务冲突及影响范围             |
| 429       | 显示请求过于频繁和稍后重试         |
| 500/503   | 显示通用服务错误、请求 ID 和重试   |

### 13.4 接口可用性矩阵

| 能力                   | 接口                                    | 当前状态   | 前端处理                         |
| ---------------------- | --------------------------------------- | ---------- | -------------------------------- |
| 注册/登录/当前用户     | `/auth/register`、`/auth/login`、`/me`  | 已有       | 可直接产品化                     |
| 简历上传、列表、详情   | `/resumes...`                           | 已有基础版 | 列表需升级分页，详情需补文件字段 |
| 简历重命名/删除/重解析 | `/resumes/:id...`                       | 待开发     | 功能标记阻塞                     |
| JD 创建/列表           | `/job-descriptions`                     | 已有基础版 | 列表需升级分页                   |
| JD 详情/更新/删除      | `/job-descriptions/:id`                 | 待开发     | 功能标记阻塞                     |
| 题集生成               | `POST /question-sets`                   | 已有       | 可直接产品化                     |
| 题集管理               | `/question-sets...`                     | 待开发     | 功能标记阻塞                     |
| 创建/获取面试          | `/interviews`、`/interviews/:id`        | 已有基础版 | 需补计时和摘要字段               |
| 顺序回答               | `POST /interviews/:id/answer`           | 已有旧版   | 仅兼容，不满足正式体验           |
| 跳题/改答/完成         | `/interviews/:id/turns...`、`/complete` | 待开发     | 面试完整交互阻塞                 |
| 报告                   | `/interviews/:id/report`                | 已有基础版 | 需补问题、原回答和时间字段       |
| 单任务                 | `/tasks/:id`                            | 已有基础版 | 上传解析可轮询                   |
| 任务列表/重试          | `/tasks`、`/tasks/:id/retry`            | 待开发     | 任务中心阻塞                     |
| 仪表盘                 | `/dashboard/summary`                    | 待开发     | 仪表盘统计阻塞                   |
| 会话刷新/退出          | `/auth/refresh`、`/auth/logout`         | 待开发     | 正式会话管理阻塞                 |

## 14. 测试 TODO

### FE-TEST-01 单元与组件测试

- [ ] 登录、注册、JD 和设置表单校验。
- [ ] API 错误归一化和字段错误映射。
- [ ] Query Key 和缓存失效。
- [ ] 主题切换及首次加载无闪烁。
- [ ] 面试单题和整场计时计算。
- [ ] 跳过、改答、完成和锁定状态转换。
- [ ] 本地草稿隔离、恢复和清理。
- [ ] 报告字段、空答案和证据展示。

### FE-TEST-02 端到端测试

- [ ] 注册并进入仪表盘。
- [ ] 登录失败、登录成功和安全返回原页面。
- [ ] 上传简历并看到解析结果。
- [ ] 创建、编辑和删除 JD。
- [ ] 生成、编辑和重新生成题集。
- [ ] 创建面试、跳过、返回作答、覆盖回答并完成。
- [ ] 刷新面试页面后恢复回答、进度和计时。
- [ ] 存在未回答题时完成确认。
- [ ] 完成后不能修改答案。
- [ ] 查看并打印报告。
- [ ] 320px 移动端完成核心流程。
- [ ] Access Token 失效后刷新或重新登录并返回原页面。

### FE-TEST-03 视觉、性能和可访问性

- [ ] 官网、仪表盘、简历列表、面试房间、报告页截图基线。
- [ ] 每个基线覆盖桌面/移动端和浅色/深色。
- [ ] 官网 Lighthouse 桌面和移动性能不低于 85。
- [ ] 核心页面 Lighthouse 可访问性不低于 90。
- [ ] 键盘完成登录、上传、创建 JD、面试和查看报告。
- [ ] 无明显布局跳动，大型报告和长列表按需加载。

## 15. 分阶段交付

### Phase 1：基础与产品壳

- [ ] FE-BASE 全部任务。
- [ ] FE-UI 全部任务。
- [ ] 官网、登录和注册。
- [ ] 对应单元测试和视觉基线。

### Phase 2：现有能力产品化

- [ ] 简历上传、列表和详情。
- [ ] JD 创建和列表。
- [ ] 题集生成。
- [ ] 基础面试和报告。
- [ ] 单任务状态展示。

### Phase 3：完整资源与面试交互

- [ ] 简历、JD、题集完整管理。
- [ ] 面试历史。
- [ ] 跳题、返回、改答、主动完成。
- [ ] 服务端计时恢复和断网草稿。

### Phase 4：数据闭环与质量

- [ ] 仪表盘。
- [ ] 任务中心。
- [ ] 设置和正式会话管理。
- [ ] 全量 E2E、视觉回归、性能和可访问性验收。

## 16. 最终验收清单

- [ ] 未登录访问 `/` 为完整中文产品官网。
- [ ] 19 个确认页面均有独立路由。
- [ ] 注册、登录、刷新会话、退出和鉴权跳转完整。
- [ ] 用户可完成“简历 → JD → 题集 → 面试 → 报告”闭环。
- [ ] 面试一次只显示一道题并同时显示单题和整场计时。
- [ ] 完成前可跳过、返回和覆盖回答，完成后完全锁定。
- [ ] 所有资源和异步操作均有完整状态反馈。
- [ ] 浅色和深色主题均通过视觉验收。
- [ ] 320px 下无横向滚动且可完成核心流程。
- [ ] 不展示 Beta 面经和 ASR 正式入口。
- [ ] TypeScript 不存在未解释的 `any`。
- [ ] `pnpm lint`、`pnpm typecheck`、`pnpm test` 和 `pnpm build` 全部通过。
- [ ] 核心页面通过键盘操作和基本可访问性检查。
