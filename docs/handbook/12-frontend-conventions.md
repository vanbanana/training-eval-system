# 12 前端开发约定

本手册描述前端项目的核心约定、目录组织、组件分层、主题与暗色模式实现等。

## 技术栈

| 层次 | 选型 |
|------|------|
| 框架 | Vue 3 (Composition API + `<script setup>`) |
| 构建 | Vite 5 |
| 语言 | TypeScript（strict mode） |
| UI 库 | shadcn-vue（基于 Reka UI） |
| 样式 | Tailwind CSS + tailwind-merge |
| 表格 | TanStack Table v8 |
| 文件上传 | filepond-vue |
| 图标 | Lucide-vue-next |
| 图表 | ECharts |
| 状态管理 | Pinia |
| 路由 | Vue Router 4 |
| HTTP | Axios + OpenAPI 自动生成 client |
| 工具集 | VueUse |
| 测试 | Vitest（单元）、Playwright（e2e） |

## 组件分层

前端组件分为三层，**禁止跨层反向依赖**：

```
business/   业务组件（FileUploader, DataTable, ChatDialog 等）
   ↓ 组合
ui/         shadcn-vue 基础组件（Button, Card, Dialog 等）
   ↓ 基于
Reka UI     无头逻辑（开发者一般不直接接触）
```

### `components/ui/`

- 由 `npx shadcn-vue add <name>` 命令拷贝进来的组件源码
- 每个组件一个目录，含 `.vue` 文件 + `index.ts` 导出
- **可以直接修改源码**（这是 shadcn 的核心理念），但应尽量保持与官方一致以方便后续升级
- 修改前先 commit 一次原版作为对比基线

### `components/business/`

- 业务相关的复合组件，**只能 import `ui/` 和 `lib/`**
- 命名采用大驼峰，文件名与组件名一致
- 每个组件应有 props 类型定义（TS interface），尽量不依赖全局 store
- 复杂业务组件应在 Storybook 或 `views/_dev/` 下提供独立预览页

### `views/`

- 路由页面，按角色分子目录
- 页面组件命名以 `View` 结尾，如 `LoginView.vue`、`DashboardView.vue`
- 页面只做"路由层胶水"，复杂逻辑下沉到 composables 或 stores

## Tailwind 使用规范

### 主题变量

shadcn-vue 使用 CSS 变量定义颜色，所有组件通过 `bg-background / text-foreground / border-border` 等语义化类引用，**禁止直接写 `bg-white / bg-gray-100` 等具体颜色**。

```css
/* src/styles/globals.css */
@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222 47% 11%;
    --primary: 221 83% 53%;
    --muted: 210 40% 96%;
    /* ... */
  }
  .dark {
    --background: 222 47% 11%;
    --foreground: 213 31% 91%;
    /* ... */
  }
}
```

### `cn()` 工具函数

shadcn-vue 标配的类名合并工具，必须用它处理动态类名：

```ts
// src/lib/utils.ts
import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

```vue
<!-- 使用示例 -->
<button :class="cn('px-4 py-2', isActive && 'bg-primary text-primary-foreground', extraClass)">
  Click
</button>
```

### 常用类名约定

| 场景 | 推荐类名 |
|------|---------|
| 卡片 | `rounded-lg border bg-card text-card-foreground shadow-sm` |
| 主操作按钮 | `bg-primary text-primary-foreground hover:bg-primary/90` |
| 次要按钮 | `bg-secondary text-secondary-foreground hover:bg-secondary/80` |
| 危险按钮 | `bg-destructive text-destructive-foreground hover:bg-destructive/90` |
| 弱化文字 | `text-muted-foreground` |
| 边框 | `border border-border` |
| 圆角 | `rounded-md`（小）/ `rounded-lg`（中）/ `rounded-xl`（大） |
| 间距 | 优先 `gap-` 而非 `space-x-`/`space-y-` |
| 响应式 | 移动端优先，`md: lg: xl:` 渐进增强 |

## Dark 模式实现

### 切换机制

使用 VueUse 的 `useColorMode`，自动持久化到 localStorage：

```ts
// composables/useTheme.ts
import { useColorMode } from "@vueuse/core"

export const useTheme = () => useColorMode({
  attribute: "class",
  modes: { light: "", dark: "dark" },
  storageKey: "tes-theme",
})
```

切换器组件挂在顶栏，点击切换 `<html>` 上的 `dark` 类，所有 `dark:` 前缀的 Tailwind 类自动生效。

### 编写规则

- 所有自定义样式必须同时考虑两种主题
- 使用语义化变量（`bg-background`）而非具体颜色，自动适配
- 自定义组件中如需特定颜色，用 `dark:` 前缀双写

```vue
<!-- 反面 -->
<div class="bg-white text-black">
<!-- 正面 -->
<div class="bg-background text-foreground">
<!-- 必要时双写 -->
<div class="bg-blue-50 dark:bg-blue-950">
```

## 状态管理（Pinia）

### Store 命名

- 文件名：`{domain}.ts`，如 `auth.ts`、`notification.ts`
- 导出函数：`use{Domain}Store`，如 `useAuthStore`

### 职责划分

- **Pinia store**：跨页面共享状态（用户信息、未读通知、主题）
- **Composables**：单页或局部状态（表单、WebSocket 连接、轮询）
- **组件 props/emit**：父子通信
- **provide/inject**：仅在深层嵌套且无其他选项时使用

### 示例

```ts
// stores/auth.ts
export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null)
  const token = useStorage("tes-token", "")
  
  const isAuthenticated = computed(() => !!token.value)
  
  async function login(username: string, password: string) {
    const res = await api.auth.login({ username, password })
    token.value = res.access_token
    user.value = res.user
  }
  
  return { user, token, isAuthenticated, login }
})
```

## 路由与权限

### 角色守卫

```ts
// router/guards.ts
export const roleGuard: NavigationGuard = (to, from, next) => {
  const auth = useAuthStore()
  const requiredRoles = to.meta.roles as string[] | undefined
  
  if (!requiredRoles) return next()
  if (!auth.isAuthenticated) return next("/login")
  if (!requiredRoles.includes(auth.user!.role)) return next("/403")
  next()
}
```

### 路由元信息

```ts
{
  path: "/teacher/tasks",
  component: TaskListView,
  meta: {
    roles: ["teacher", "admin"],
    breadcrumb: ["教师工作台", "实训任务"],
  }
}
```

## API 调用

### 统一 client

由 `openapi-typescript-codegen` 工具基于后端 OpenAPI schema 自动生成，**严禁手写接口字段**：

```bash
# package.json
"scripts": {
  "gen:api": "openapi --input http://localhost:8000/openapi.json --output src/api/generated --client axios"
}
```

### 拦截器

`src/api/client.ts` 封装统一拦截器：

- 请求：自动注入 `Authorization: Bearer <token>` 与 `X-Trace-Id`
- 响应：401 → 登出跳登录；403 → 弹 toast；5xx → 上报错误追踪
- 错误统一抛 `ApiError` 类型，含 `error_code / message / field`

## 表格（TanStack Table + shadcn）

shadcn-vue 官方推荐组合，业务组件 `DataTable.vue` 包装如下：

```vue
<!-- 调用方式 -->
<DataTable
  :columns="columns"
  :data="rows"
  :pagination="{ pageIndex: 0, pageSize: 20 }"
  enable-sorting
  enable-filtering
  @row-click="onRowClick"
/>
```

复杂表格（如批改工作台）可直接基于 TanStack Table 自定义。

## 文件上传

使用 `FileUploader.vue` 业务组件包装 filepond-vue：

```vue
<FileUploader
  :accept="['.docx', '.pdf', '.png', '.jpg']"
  :max-size-mb="50"
  :chunk-upload="true"
  :allow-multiple="true"
  endpoint="/api/uploads"
  @success="onUploadSuccess"
  @error="onUploadError"
/>
```

支持断点续传与 SHA256 校验，对应需求 4.8。

## 实时推送（WebSocket）

封装 `useWebSocket` composable，自动处理重连：

```ts
// composables/useWebSocket.ts
export function useWebSocket(channel: "progress" | "notify" | "chat", token: string) {
  const ws = ref<WebSocket | null>(null)
  const messages = ref<unknown[]>([])
  
  function connect() {
    ws.value = new WebSocket(`${WS_BASE}/ws/${channel}/${token}`)
    ws.value.onmessage = (e) => messages.value.push(JSON.parse(e.data))
    ws.value.onclose = () => setTimeout(connect, 3000)  // 自动重连
  }
  
  onMounted(connect)
  onUnmounted(() => ws.value?.close())
  
  return { messages }
}
```

## 测试

### 单元测试（Vitest）

测试纯组件、composables、utils，对外部依赖（fetch、router、store）使用 mock：

```ts
import { mount } from "@vue/test-utils"
import { describe, it, expect } from "vitest"
import Button from "@/components/ui/button/Button.vue"

describe("Button", () => {
  it("renders slot content", () => {
    const wrapper = mount(Button, { slots: { default: "Click" } })
    expect(wrapper.text()).toBe("Click")
  })
})
```

### E2E 测试（Playwright）

覆盖 5 条关键用户路径：登录 → 上传 → 解析等待 → 查看评分 → 导出 PDF。每条路径录屏归档供答辩演示。

## 性能与可访问性

- **路由懒加载**：`component: () => import("./LoginView.vue")`
- **图片优化**：使用 `<img loading="lazy">`，按需引入字体子集
- **可访问性**：shadcn-vue 默认提供完整 ARIA，业务组件需保留 `aria-label`、键盘导航
- **国际化**：使用 vue-i18n 集中管理文案，避免组件内硬编码中文

## PR 自检清单（前端）

- [ ] 没有写死颜色（`bg-white / #fff` 等），都用语义化变量
- [ ] 所有交互组件支持 dark 模式
- [ ] API 调用使用 generated client，未手写字段
- [ ] 组件类型完整（props/emits/slots 都有 TS 类型）
- [ ] `pnpm lint` 与 `pnpm typecheck` 通过
- [ ] 关键交互有 Vitest 单元测试覆盖
- [ ] 移动端 1280px 以下不产生水平滚动条
- [ ] 长耗时操作有 loading 指示器
