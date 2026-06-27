# Agent 页面优化方案 — Phase 4 设计方向

> 基于 skill: design-character.md → AI 产品 Boldness=6, Motion=7, Density=5
> 基于 skill: typography.md → 字号三级体系
> 基于 skill: visual-rhythm.md → 间距变化
> 基于 skill: animation-discipline.md → 动效规则
> 基于 skill: anti-ai-slop.md → P0 检查
> 基于 skill: accessibility-baseline.md → 语义化
> 基于 skill: components.md → UI 完整性

## 改造任务分解

### Task 1: 全局基础设施 (globals.css)
- 添加 `--code-bg` / `--code-fg` CSS 变量
- 添加统一 prose 样式（消除 3 处重复）
- 添加 `.anim-message-enter` 消息入场动效类
- 添加 `prefers-reduced-motion` 无障碍回退
- 删除/禁用冲突的 style.css

### Task 2: Teacher AgentView
- 字号全线提升（badge/timestamp/tool/input hint → 14px/12px）
- 硬编码 `#1e293b` → `var(--code-bg)`
- 删除重复 prose 样式块
- 视觉节奏调整（section 间距变化）
- 消息入场动效

### Task 3: Admin AgentView  
- 同步 Task 2 所有改动

### Task 4: Student ChatView
- 同步 Task 2 所有改动

### Task 5: ChatDialog
- 修复硬编码样式（渐变背景、固定高度）
- 字号修复
- 主题变量对齐