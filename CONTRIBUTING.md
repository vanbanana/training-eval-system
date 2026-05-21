# 贡献指南

## 分支策略

- `main`：受保护，仅通过 PR 合入。
- `feat/<scope>-<short-name>`：功能开发。
- `fix/<scope>-<short-name>`：bug 修复。
- `docs/<short-name>`：纯文档变更。
- `chore/<short-name>`：构建/CI/工具链。

## 提交规范（Conventional Commits）

强制使用以下类型：

| 类型 | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | bug 修复 |
| `docs` | 文档 |
| `test` | 测试 |
| `chore` | 杂务（依赖升级、CI 配置等） |
| `refactor` | 重构（无功能变化） |
| `perf` | 性能优化 |
| `build` | 构建系统 |
| `ci` | CI 流程 |
| `style` | 仅格式（空格、分号等） |

示例：

```
feat(auth): 添加 JWT 刷新令牌端点
fix(grading): 修复教师分数覆盖未触发综合分重算
docs(handbook): 补充 LLM Skills 注册流程
test(parsers): docx 解析支持 6 级标题嵌套
```

非法 commit 消息（如 `update files`）会被 `commit-msg` 钩子拒绝。

## PR 规范

### PR 标题

格式：`<type>(<scope>): <summary>`，控制在 70 字符内。

### PR 描述模板

```markdown
## 变更概述
<!-- 一句话说明做了什么 -->

## 关联 Task
<!-- 例如：Task 5.3 / Task 5.5 -->

## 测试与验证
- [ ] `./scripts/lint_all.sh` 全绿
- [ ] 单元测试新增/更新 N 条
- [ ] （前端）`pnpm build` 通过
- [ ] （前端）视觉对照 HTML 参考无 5px 偏差

## 不向后兼容变更
<!-- 如有数据库迁移、API 变更，列出 -->

## 截图（前端）
<!-- 如涉及 UI 变更 -->
```

## 代码风格

### Python（后端）

- 100% 类型注解，`mypy --strict` 通过
- ruff lint + format 通过（双引号 + LF 行尾 + 空格缩进）
- 公开函数必须有 docstring
- 业务异常必须继承 `BusinessError`，禁止裸 `raise Exception`

### TypeScript（前端）

- 启用 `strict: true`
- 禁止 `any` 透传业务数据
- 组件文件 PascalCase（`UsersView.vue`）
- 工具函数 camelCase

### 文件大小

- 单文件 ≤ 500 行
- 单函数 ≤ 60 行
- 单类 ≤ 200 行

如需超过，拆分前先讨论。

## 测试要求

- 核心算法（评分、相似度、权重校验）覆盖率 100%
- 其他模块覆盖率 ≥ 70%
- 测试数据**必须**通过 Factory 生成，禁止字面值

详见 [`docs/handbook/10-testing-and-dev-endpoints.md`](docs/handbook/10-testing-and-dev-endpoints.md)。

## 安全

- 禁止提交 API Key / 密码 / 私钥到 git（`detect-secrets` 钩子拦截）
- `.env` 永远不要入库，仅入 `.env.example`
- LLM API Key 必须用 AES-256-GCM 加密存储

## 不要做的事

- ❌ 直接推 `main`
- ❌ 用 `--no-verify` 跳过 pre-commit（除紧急 hotfix 后立即补测试）
- ❌ 在 commit 中混入多个类型的变更（一次只做一件事）
- ❌ 不写测试就提交业务代码
- ❌ 修改设计稿瑕疵记录列出的弃用项（见 [`docs/handbook/12-frontend-conventions.md`](docs/handbook/12-frontend-conventions.md)）
