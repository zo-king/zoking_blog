# 子 Agent 协作规则

本文件定义后续多 agent 干活方式。当前对话窗口是中心调度，不把最终决策外包给子 agent。

## 总原则

- 中心窗口负责汇报、调度、整合、记录和验收。
- 子 agent 只处理明确分派的任务。
- 每个子 agent 必须有清晰输入、输出、文件范围和验收方式。
- 子 agent 不得擅自扩大范围。
- 子 agent 不得回滚或覆盖其他 agent / 用户的改动。
- 所有关键决策必须写入工作日志。
- 上下文过大时，必须先更新工作日志和接力文档，再开新窗口。

## 中心窗口职责

中心窗口负责：

- 拆解需求。
- 判断哪些任务可以并行。
- 分配 agent 角色。
- 控制文件写入边界。
- 避免重复调研和重复实现。
- 收集子 agent 结果。
- 统一写入文档。
- 维护 `docs/process/worklog.md`。
- 更新 `docs/process/context-handoff.md`。
- 汇报当前阶段进展。

## 推荐 Agent 角色

| 角色 | 职责 | 默认写入范围 |
|---|---|---|
| 需求 agent | 需求来源、角色、场景、范围、验收 | `docs/requirements/*` |
| 主题分析 agent | Stack 能力、源码结构、配置映射 | 通常只读；必要时写 `docs/requirements/02-stack-capability-map.md` |
| 内容架构 agent | 内容目录、frontmatter、taxonomy、archetypes | `content/*`、`archetypes/*`、内容规范文档 |
| 配置 agent | Hugo 和 Stack 配置 | `config/_default/*` |
| 前端定制 agent | 视觉、SCSS、少量 TS | `assets/scss/custom.scss`、`assets/ts/custom.ts` |
| 部署 agent | CI/CD、部署平台、runbook | `.github/*`、`wrangler.toml`、部署文档 |
| QA agent | 构建、浏览器检查、移动端、可访问性 | 通常只读；写验证报告 |
| 文档 agent | README、工作日志、接力文档 | `docs/*` |

## 任务分派模板

```markdown
任务：

背景：

允许编辑：

禁止编辑：

输入资料：

预期产物：

验证方式：

汇报要求：
```

## 子 Agent 汇报模板

```markdown
完成状态：

读取资料：

主要结论：

修改文件：

验证命令：

验证结果：

风险：

建议下一步：
```

## 文件冲突控制

- 同一时间不要让多个 agent 写同一个文件。
- 如果必须多人参与同一文档，先让 agent 输出结论，由中心窗口统一写入。
- 配置、内容、前端、部署、文档尽量分离。
- 中心窗口在合并前必须检查 `git status --short --branch`。
- 写入型任务必须同步更新 `docs/process/task-board.md` 的文件锁。

## 文件锁登记规则

中心窗口派发写入型任务时必须登记文件锁：

| 任务编号 | Agent | 允许编辑 | 禁止编辑 | 锁定时间 | 释放条件 |
|---|---|---|---|---|---|

规则：

- 一个文件同一时间只能被一个写入型任务锁定。
- 只读审阅任务不占用文件锁，但必须声明只读。
- 若需要多人影响同一文件，子 agent 只输出建议，由中心窗口统一写入。
- agent 完成汇报后，中心窗口验收并释放文件锁。
- 发现未登记改动时，中心窗口必须先判断是否为用户改动，不得回滚。

## 并行合并协议

子 agent 完成后，中心窗口按顺序处理：

1. 阅读子 agent 汇报。
2. 检查修改文件是否在允许范围。
3. 检查 `git status --short --branch`。
4. 运行任务约定验证命令。
5. 将结论写入 `docs/process/worklog.md`。
6. 验收通过后释放文件锁。
7. 不合格时把任务置为 `Blocked` 或重新派发。

## 主题源码保护规则

默认禁止直接修改：

- `layouts/baseof.html`
- `layouts/home.html`
- `layouts/single.html`
- `assets/scss/style.scss`
- `assets/scss/variables.scss`
- `assets/ts/main.ts`
- `layouts/_partials/helper/*`

优先使用：

- `config/_default/*`
- `assets/scss/custom.scss`
- `assets/ts/custom.ts`
- `layouts/_partials/head/custom.html`
- `layouts/_partials/footer/custom.html`
- 站点层同路径覆盖

## 开新窗口规则

触发条件：

- 当前上下文过长。
- 阶段完成，需要交接。
- 中心窗口判断继续工作可能丢失重要决策。

开新窗口前必须：

1. 更新 [工作日志](worklog.md) 顶部状态摘要。
2. 更新 [当前任务看板与文件锁](task-board.md)。
3. 更新 [上下文接力规则](context-handoff.md) 的“当前状态”和“下一步”。
4. 确认 `git status --short --branch`。
5. 在最终汇报中说明新窗口要读哪些文件。
