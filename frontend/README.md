# 智能实训评价管理系统 · 前端

Vue 3 + TypeScript + Vite + shadcn-vue（Reka UI）+ Tailwind CSS v4。

## 启动

```bash
pnpm install
pnpm dev      # http://localhost:5173
pnpm build    # 输出 dist/
```

## 组件添加规则（强约束）

新增 UI 组件**优先**使用：

```bash
pnpm dlx shadcn-vue@latest add <name>
```

**禁止**手搓 Modal / Dropdown / Popover / Toast / Sheet —— 全部从 `@/components/ui/*` 引入。

如果 shadcn-vue 没有合适组件（如 GlobalSearch、DataTable），放到 `@/components/business/` 并基于 `@/components/ui/` 组合。

## 关键基础设施

| 文件 | 用途 |
|---|---|
| `src/lib/utils.ts` | `cn()` 类名合并工具（clsx + tailwind-merge） |
| `src/styles/globals.css` | 主题变量（HSL）+ Tailwind v4 `@theme inline` 映射 |
| `components.json` | shadcn-vue CLI 配置 |
| `src/composables/useTheme.ts` | 暗色模式切换（VueUse `useColorMode`） |
| `src/composables/useConfirm.ts` | 全局 confirm 服务（替代原生 `confirm()`） |
| `src/components/ui/toast/` | shadcn 风格 Toast（`useToast()` / `<Toaster />`） |

## 视觉契约

- 颜色 / 字号 / 圆角 / 间距：用 Tailwind class，不硬编码 hex
- 1:1 视觉对比 `frontend-preview/pages/*.html`，5px 内偏差为合格
- 详细规则见 `.kiro/steering/frontend-rules.md` 与 `docs/handbook/12-frontend-conventions.md`

## 动效约束

- 路由切换：`fade` transition（200ms）
- 列表 / 卡片入场：`anim-in` 工具类（CSS `@starting-style`）+ stagger 50ms
- 数字递增：`@/components/business/AnimatedNumber.vue`（基于 `@vueuse/core` `useTransition`）
- 模态 / Popover / Dropdown / Sheet 开关：reka-ui 内置 + `tw-animate-css` 提供的 fade-in / zoom-in / slide-in
- Toast 进出：slide-in-from-top + fade
- 按钮 hover/active：`scale(0.98)` + 阴影微动
- **禁止**：parallax、全局 `*` transition、> 400ms 动画、Lottie / 大型 SVG 动画

## 目录结构

```
src/
  api/           Axios + 拦截器
  components/
    ui/          shadcn-vue 原子组件（17 个 UI 包）
    business/   业务复合组件（DataTable / EmptyState / ConfirmDialog / GlobalSearch / AnimatedNumber / MotionList / BreadcrumbNav）
    layout/     AppShell / TopNav
  composables/   useTheme / useConfirm / useNotifications
  lib/           utils (cn) / toast 兼容层
  router/        vue-router 4
  stores/        Pinia
  styles/        globals.css
  views/         按角色分子目录（admin / teacher / student / shared / auth）
```
