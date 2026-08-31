# docs 文档地图

> 整理日期：2026-08-28。先看本页再打开具体文档，避免把归档清单当当前任务。

当前执行清单是 [`邀请制todo_0828.md`](邀请制todo_0828.md)。智能层设计看 [`agent设计.md`](agent设计.md)，本地怎么跑看 [`本地启动.md`](本地启动.md)。

## 当前有效

| 文档 | 用途 | 注意 |
| --- | --- | --- |
| [`邀请制todo_0828.md`](邀请制todo_0828.md) | 邀请制上线还剩什么 | **唯一执行清单**；不要再往 `上线TODO.md` / `agent_todo.md` 加新任务 |
| [`agent设计.md`](agent设计.md) | 智能层原则、边界、状态机 | 设计源；不要在这里堆实施勾选 |
| [`Agent架构与接口改造_TODO_0814.md`](Agent架构与接口改造_TODO_0814.md) | Agent V2 接口、迁移、Worker 改造说明 | **主体已落地**（库表 v10、`interview:next_turn`、聊天室）。文内大量 `[ ]` 未回写，剩余项以邀请制 TODO 为准 |
| [`上线重构_0814.md`](上线重构_0814.md) | 产品主线：无 JD/题集管理，面试是用户单位 | 产品边界仍有效 |
| [`技术方案_v1.md`](技术方案_v1.md) | 工程栈与仓库结构 | 2026-07-18 基线；产品流程已被 0814 覆盖，栈（Go / Next / PG / Redis / MinIO）仍准 |
| [`本地启动.md`](本地启动.md) | 本机 Compose、迁移、API/Worker/Web | 2026-08-15 在 Windows 验证过 |
| [`runbooks/ai.md`](runbooks/ai.md) | 模型故障排查、降级、`IM_AI_ENABLED` | 运维入口 |

根目录 [`README.md`](../README.md) 只保留产品一句话、栈和最短启动命令；细节以本目录为准。

## 已归档（`archive/`）

过期任务清单、旧产品化 TODO、OceanBase / 多微服务草稿。完整说明见 [`archive/README.md`](archive/README.md)。

不要按这些文件排期或验收。需要对照「当时怎么想的」时再打开。

## 整理结论（删 / 合 / 留）

**没有直接删除。** 这些文件仍有历史对照价值；删了以后搜索和 PR 讨论会断链。过期内容一律进 `archive/`，并在文件头标明「不要当当前任务」。

**没有把多份长文档合并成一篇。** `全流程开发大纲`、`开发排期表`、`需求功能大纲` 彼此重叠，但都是过时选型（OceanBase、LangChainGo、七微服务、JD/RAG）。合成一篇只会更难读。`frontend-refactor-*` 与 `backend-refactor-todo` 同理：那是 07-29 的 JD + 题集产品化，已被 0814 取代。

**可以当作「已完成、不要再执行」的：**

| 文件 | 原因 |
| --- | --- |
| `archive/上线TODO.md` | 2026-08-07，仍写「AI 全部为 stub」，与现状相反，**最危险** |
| `archive/验收清单.md` | M0–M3/Beta，含创建 JD、固定五题、SHA-256 向量 |
| `archive/frontend-refactor-*.md`、`backend-refactor-todo.md` | 07-29 产品化，主流程已过时；现行契约是 `backend/api/interviewmaster.api` |
| `archive/agent_todo.md` | Agent 基建大多 `[x]`；未完成项（邀请门禁、Golden/E2E 门禁）已收到邀请制 TODO |
| `archive/全流程开发大纲.md`、`开发排期表_Phase0_1.md`、`需求功能大纲.md` | 明确历史草稿，文内已写「请勿作为当前基线」 |
| `archive/PROJECT_WALKTHROUGH.md` | 08-06 通读，仍按 JD/题集讲 |

**建议以后谁来改文档：**

1. 新任务只写进 `邀请制todo_0828.md`，或另开 `docs/yyyyMMdd_主题.md`，并在本页「当前有效」加一行。
2. 设计变更改 `agent设计.md`，不要复制进 TODO。
3. 勾选了就改对应文件；不要让第二份清单和代码各说各话。
