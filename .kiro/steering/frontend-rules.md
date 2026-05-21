---
inclusion: fileMatch
fileMatchPattern: 'frontend/**/*'
---

# 前端开发规则

## 复刻流程（强约束）

任意 `frontend/src/views/**/*.vue` 或 `frontend/src/components/**/*.vue` 编写前必须：

1. 读 `frontend-preview/tokens.css`（颜色 / 圆角 / 字体变量）
2. 读 `frontend-preview/shared.css`（TopNav / Card / Button / Label / Table / Input 公共类）
3. 在 `docs/design/02-html-references.md` 找到对应 View 的 HTML 参考路径，读该 HTML 文件作为视觉契约

## 实现规则

- 用 **shadcn-vue + Tailwind class** 复刻；**禁止**直接拷贝 HTML 内联 `<style>` 或硬编码颜色
- 颜色 / 字号 / 圆角 / 间距：用 Tailwind class，对应映射见 `docs/design/01-design-tokens.md`
- 文案：以 HTML 参考的中文为初始 i18n key 源，不发明新文案
- Lucide 图标名：与 HTML 中 `icon-*` 完全一致
- DOM 层级、模块顺序、卡片粒度：与 HTML 参考一致；偏离需 PR 描述写明原因
- HTML 中的所有静态状态（hover / active / empty / error）必须能通过 prop 或 state 切换到

## 验收

- 1:1 视觉对比 HTML 参考与 Vue 实现，5px 内偏差视为合格
- 如发现 HTML 参考有 bug，**先改 HTML，再实现 Vue**

## 设计稿瑕疵记录（不复刻）

- 弃用 sidebar 系列，统一用 TopNav 顶部双行导航
- 弃用 `--font-serif` / `--font-cn`，仅保留 `--font-sans` 与 `--font-mono`
- 第 25 页"解析进度"合并入 GradingView 子组件
- 第 30 页"任务详情"与第 8 页"上传"合并为 TaskDetailView
- 第 29 页"学校级"与第 12 页"教学画像"合并为 ProfileView，scope 参数切换
