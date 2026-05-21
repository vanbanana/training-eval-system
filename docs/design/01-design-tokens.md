# 01 设计 Token

源自 Pencil 设计稿 `designs/training-evaluation2.pen` 的 design variables，已修正瑕疵。直接对应 Tailwind 配置 + shadcn-vue 主题变量。

## 颜色系统

### 中性色（基础）

| Token | Hex | 用途 |
|-------|-----|------|
| `--background` | `#F4F5F7` | 页面背景（body） |
| `--surface` | `#FFFFFF` | 卡片/导航/面板表面 |
| `--surface-2` | `#F9FAFB` | 表头/次级背景 |
| `--surface-3` | `#F2EEE3` | 强调区背景（如 stat 卡） |
| `--border` | `#E5E7EB` | 默认边框 |
| `--border-soft` | `#EEEAE0` | 弱化边框（用于 surface-3 内） |
| `--border-strong` | `#CBD0D8` | 强调边框 |
| `--ink` | `#0F1B2D` | 标题/重要文字 |
| `--foreground` | `#1F2937` | 正文文字 |
| `--muted-foreground` | `#5B6473` | 次要文字 |
| `--subtle-foreground` | `#8E96A4` | 提示/icon 灰 |

### 品牌主色（学院深蓝）

| Token | Hex | 用途 |
|-------|-----|------|
| `--primary` | `#1E3A5F` | 主操作按钮、登录页大色块 |
| `--primary-strong` | `#172B47` | hover 态、深色装饰 |
| `--primary-soft` | `#E5ECF4` | 主色背景化（avatar 底） |
| `--primary-foreground` | `#FFFFFF` | 主色块上的文字 |

### 强调色（暖橙）

| Token | Hex | 用途 |
|-------|-----|------|
| `--accent` | `#C2410C` | 副 CTA、链接、关键标签 |
| `--accent-soft` | `#FBE5D6` | 标签底色 |
| `--accent-strong` | `#92400E` | 强调文字 |

### 状态色

| Token | Hex | 用途 |
|-------|-----|------|
| `--success` | `#15803D` | 成功状态 |
| `--success-soft` | `#E6F4EC` | 成功标签底 |
| `--warning` | `#B45309` | 警告状态 |
| `--warning-soft` | `#FBE8CF` | 警告标签底 |
| `--danger` | `#B91C1C` | 危险状态 |
| `--danger-soft` | `#FBECEC` | 危险标签底 |
| `--info` | `#1D4ED8` | 信息提示 |
| `--info-soft` | `#E5EBF7` | 信息标签底 |
| `--gold` | `#A47B2A` | 系统模板/优秀奖项 |
| `--gold-soft` | `#F5EDD9` | 金标签底 |

### 弃用色（设计稿存在但项目不用）

- `--sidebar-*` 系列：项目使用顶部导航，不实现侧边栏

## 字体

| Token | 值 | 用途 | Tailwind 名称 |
|-------|---|------|--------------|
| `--font-sans` | `Noto Sans SC, system-ui, sans-serif` | 全局正文与标题 | `font-sans` |
| `--font-mono` | `JetBrains Mono, Cascadia Code, Consolas, monospace` | 代码、密钥、ID | `font-mono` |

> 项目所有文字均用 sans，弃用 `--font-serif` / `--font-cn`。

## 字号阶梯

| Tailwind | px | 用途 |
|----------|----|------|
| `text-xs` | 12 | 面包屑、标签、辅助 |
| `text-sm` | 13 | 表格内容、卡片描述 |
| `text-base` | 14 | 正文、按钮 |
| `text-md` | 15 | 区块标题 |
| `text-lg` | 16 | 卡片标题 |
| `text-xl` | 18 | 二级页面标题 |
| `text-2xl` | 22 | 一级页面标题 |
| `text-3xl` | 28-30 | 登录页/品牌标题 |

## 圆角

| Token | px | Tailwind | 用途 |
|-------|----|---------|------|
| `--radius-sm` | 4 | `rounded-sm` | 标签、徽章 |
| `--radius-m` | 6 | `rounded-md` | 输入框、按钮 |
| `--radius-lg` | 10 | `rounded-lg` | 卡片、面板 |
| `--radius-xl` | 14 | `rounded-xl` | 大型容器 |
| `--radius-pill` | 999 | `rounded-full` | Avatar、状态点 |

## 间距规则

| Tailwind | px | 典型用途 |
|----------|----|---------|
| `gap-1` | 4 | icon 与文字 |
| `gap-2` | 8 | 紧凑列表 |
| `gap-3` | 12 | 表单元素之间 |
| `gap-4` | 16 | 卡片内部分组 |
| `gap-5` | 20 | 区块之间 |
| `gap-6` | 24 | 主区与辅区 |
| `gap-8` | 32 | 大区块之间 |
| `p-4` / `p-6` / `p-8` | - | 卡片/面板/页面 padding |

页面 body 默认 padding：`px-8 py-7`（参见所有页 body padding [28, 32]）。

## Tailwind 配置片段

放入 `frontend/tailwind.config.ts`：

```ts
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Noto Sans SC', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Cascadia Code', 'Consolas', 'monospace'],
      },
      colors: {
        background: 'hsl(var(--background))',
        surface: { DEFAULT: 'hsl(var(--surface))', 2: 'hsl(var(--surface-2))', 3: 'hsl(var(--surface-3))' },
        border: { DEFAULT: 'hsl(var(--border))', soft: 'hsl(var(--border-soft))', strong: 'hsl(var(--border-strong))' },
        ink: 'hsl(var(--ink))',
        foreground: 'hsl(var(--foreground))',
        muted: { DEFAULT: 'hsl(var(--muted))', foreground: 'hsl(var(--muted-foreground))' },
        subtle: { foreground: 'hsl(var(--subtle-foreground))' },
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          strong: 'hsl(var(--primary-strong))',
          soft: 'hsl(var(--primary-soft))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        accent: { DEFAULT: 'hsl(var(--accent))', soft: 'hsl(var(--accent-soft))', strong: 'hsl(var(--accent-strong))' },
        success: { DEFAULT: 'hsl(var(--success))', soft: 'hsl(var(--success-soft))' },
        warning: { DEFAULT: 'hsl(var(--warning))', soft: 'hsl(var(--warning-soft))' },
        danger: { DEFAULT: 'hsl(var(--danger))', soft: 'hsl(var(--danger-soft))' },
        info: { DEFAULT: 'hsl(var(--info))', soft: 'hsl(var(--info-soft))' },
        gold: { DEFAULT: 'hsl(var(--gold))', soft: 'hsl(var(--gold-soft))' },
      },
      borderRadius: {
        sm: '4px',
        md: '6px',
        lg: '10px',
        xl: '14px',
      },
    },
  },
}
```

> 注意：上述 `hsl(var(...))` 是 shadcn-vue 标准做法（值在 `globals.css` 用 HSL 数值定义，不带函数）。下面给出 globals.css 模板。

## globals.css 模板

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    /* HSL values without hsl() wrapper (shadcn convention) */
    --background: 220 11% 96%;        /* #F4F5F7 */
    --surface: 0 0% 100%;             /* #FFFFFF */
    --surface-2: 220 14% 98%;
    --surface-3: 41 25% 92%;
    --border: 220 13% 91%;
    --border-soft: 41 32% 91%;
    --border-strong: 220 12% 82%;
    --ink: 217 51% 12%;
    --foreground: 220 17% 17%;
    --muted: 218 20% 95%;
    --muted-foreground: 218 11% 41%;
    --subtle-foreground: 220 11% 60%;
    --primary: 215 53% 25%;
    --primary-strong: 217 51% 18%;
    --primary-soft: 217 35% 93%;
    --primary-foreground: 0 0% 100%;
    --accent: 19 86% 40%;
    --accent-soft: 30 84% 91%;
    --accent-strong: 26 73% 28%;
    --success: 144 70% 32%;
    --success-soft: 138 50% 93%;
    --warning: 30 86% 36%;
    --warning-soft: 34 88% 89%;
    --danger: 0 73% 42%;
    --danger-soft: 0 75% 95%;
    --info: 224 76% 48%;
    --info-soft: 218 56% 93%;
    --gold: 38 60% 41%;
    --gold-soft: 44 60% 90%;
  }
}

@layer base {
  body {
    @apply bg-background text-foreground font-sans antialiased;
  }
}
```
