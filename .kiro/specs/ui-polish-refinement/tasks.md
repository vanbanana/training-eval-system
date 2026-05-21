# Implementation Plan: UI Polish Refinement

## Overview

将现有前端界面从功能完整但视觉扁平的后台管理系统，升级到 2025 年现代 B 端 SaaS 产品设计水准。实现路径：Design Token 扩展 → 基础组件样式覆盖 → 新业务组件开发 → SVG 插画系统 → 页面级集成 → 性能验证。

技术约束：Vue 3 + Vite + shadcn-vue + Tailwind CSS，CSS-only 动效，无新 JS 运行时依赖，CSS 增量 < 5KB gzip。

## Tasks

- [ ] 1. Design Token 扩展与基础样式层
  - [ ] 1.1. 在 globals.css 中扩展 shadow/glass/gradient/transition 变量
    - 在 `:root` 中添加 `--shadow-sm`、`--shadow-md`、`--shadow-lg` 多层柔和阴影变量
    - 添加 `--glass-blur`、`--glass-bg`、`--glass-border` 毛玻璃变量
    - 添加 `--gradient-primary`、`--gradient-accent`、`--gradient-success`、`--gradient-warning`、`--gradient-page-bg` 渐变变量
    - 添加 `--transition-fast`、`--transition-normal` 过渡变量
    - 在 `.dark` 作用域中定义所有对应的暗色主题变量值
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

  - [ ]* 1.2. 编写 Property Test：阴影透明度约束
    - **Property 1: Shadow transparency constraint**
    - 解析 globals.css 中 shadow token 的 rgba alpha 值，验证 light 模式下不超过 0.08
    - 使用 fast-check 生成随机 shadow token 名称进行验证
    - Tag: `Feature: ui-polish-refinement, Property 1: Shadow transparency constraint`
    - **Validates: Requirements 1.1**

  - [ ]* 1.3. 编写 Property Test：主题 Token 对称性
    - **Property 2: Theme token parity**
    - 解析 globals.css，验证 `:root` 中定义的每个视觉增强 token 在 `.dark` 中都有对应定义
    - Tag: `Feature: ui-polish-refinement, Property 2: Theme token parity`
    - **Validates: Requirements 1.4, 10.1**

  - [ ]* 1.4. 编写 Property Test：装饰元素透明度约束
    - **Property 4: Decorative element opacity constraint**
    - 验证 AppShell 装饰性背景元素的 opacity 不超过 0.05
    - Tag: `Feature: ui-polish-refinement, Property 4: Decorative element opacity constraint`
    - **Validates: Requirements 3.4**

- [ ] 2. 卡片与按钮组件样式升级
  - [ ] 2.1. 升级 Card 组件样式：添加柔和阴影与 hover 过渡
    - 为 Card 组件默认应用 `box-shadow: var(--shadow-sm)` 替代纯 border
    - 添加 hover 状态过渡到 `var(--shadow-md)`，duration 200ms ease-out
    - 确保 border-radius 使用 radius-lg（≥10px）
    - 保持 1px solid border 作为底层，阴影为叠加层
    - _Requirements: 2.1, 2.2, 2.4, 2.5_

  - [ ] 2.2. 升级 Button primary 样式：添加立体感与交互反馈
    - 为 primary 按钮添加 `inset 0 1px 0 rgba(255,255,255,0.12)` 顶部高光
    - hover 状态：`filter: brightness(1.05)` + shadow 扩大，150ms ease-out
    - active 状态：`transform: scale(0.97)`，100ms 过渡
    - 统一所有交互元素 transition duration 150-200ms + ease-out
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [ ]* 2.3. 编写 Property Test：交互元素过渡一致性
    - **Property 7: Interactive element transition consistency**
    - 验证所有交互元素的 transition-duration 在 150-200ms 范围内，timing function 为 ease-out
    - Tag: `Feature: ui-polish-refinement, Property 7: Interactive element transition consistency`
    - **Validates: Requirements 7.4**

  - [ ]* 2.4. 编写 Property Test：视觉效果使用 CSS 变量
    - **Property 8: Visual effects use CSS variables**
    - 验证新增视觉效果规则中的颜色和透明度值通过 CSS 自定义属性表达
    - Tag: `Feature: ui-polish-refinement, Property 8: Visual effects use CSS variables`
    - **Validates: Requirements 10.1**

- [ ] 3. Checkpoint - 基础样式验证
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 4. 新业务组件开发
  - [ ] 4.1. 创建 StatCard 组件：带渐变色条 + staggered 入场动效
    - 实现 `StatCardProps` 接口（label, value, icon, trend, accentColor, animateValue, delay）
    - 左侧 4px 渐变色条通过 `::before` 伪元素 + `var(--gradient-{accentColor})` 实现
    - 入场动效使用 CSS `@starting-style` + `transition-delay` 实现 stagger（间隔 80-120ms）
    - hover 效果：`translateY(-2px)` + shadow 从 sm 升级到 md
    - _Requirements: 2.3, 6.1, 6.2, 6.4_

  - [ ] 4.2. 创建 AnimatedNumber 组件：纯 CSS/rAF 数字滚动
    - 实现 `AnimatedNumberProps` 接口（value, duration, format）
    - 使用 `requestAnimationFrame` 驱动数字插值，duration 默认 700ms（600-800ms 范围）
    - 应用 `font-variant-numeric: tabular-nums` 防止布局抖动
    - 不引入第三方动画库
    - _Requirements: 6.3, 9.1, 9.2_

  - [ ] 4.3. 升级 EmptyState 组件：支持 illustration prop
    - 扩展 `EmptyStateProps` 接口，添加 `illustration?: Component` prop
    - 有 illustration 时渲染 SVG 插画组件，无 illustration 时回退到 icon
    - 保持标题、描述文案、操作按钮的布局结构
    - _Requirements: 4.1, 4.5_

  - [ ]* 4.4. 编写单元测试：StatCard / AnimatedNumber / EmptyState
    - 测试 StatCard 渲染渐变色条和 stagger delay
    - 测试 AnimatedNumber 在 600-800ms 内完成动画
    - 测试 EmptyState 接受 illustration prop 并正确渲染
    - _Requirements: 2.3, 4.1, 6.3_

- [ ] 5. SVG 插画系统
  - [ ] 5.1. 创建 5 个核心空状态 SVG 插画 Vue 组件
    - 创建 `components/illustrations/` 目录
    - 实现 IllustNoTasks.vue、IllustNoNotifications.vue、IllustNoEvaluations.vue、IllustNoCourses.vue、IllustNoResults.vue
    - 所有颜色使用 `currentColor` 或 `var(--...)` CSS 变量引用
    - 每个文件体积 ≤ 8KB（未压缩），不使用 `<image>` 外部引用
    - 支持 `width`/`height` prop 控制尺寸
    - _Requirements: 4.2, 4.3, 4.4, 9.5_

  - [ ]* 5.2. 编写 Property Test：SVG 主题适配性
    - **Property 5: SVG theme adaptability**
    - 扫描 illustrations 目录下所有 SVG 组件，验证 fill/stroke 不含硬编码 hex/rgb 值
    - Tag: `Feature: ui-polish-refinement, Property 5: SVG theme adaptability`
    - **Validates: Requirements 4.3**

  - [ ]* 5.3. 编写 Property Test：SVG 文件大小预算
    - **Property 6: SVG file size budget**
    - 验证每个 SVG 插画组件文件未压缩大小不超过 8KB (8192 bytes)
    - Tag: `Feature: ui-polish-refinement, Property 6: SVG file size budget`
    - **Validates: Requirements 4.4**

- [ ] 6. Checkpoint - 组件与插画验证
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 7. 布局与导航升级
  - [ ] 7.1. 升级 TopNav 组件：应用 glassmorphism 效果
    - 设置 `background: var(--glass-bg)` + `backdrop-filter: blur(var(--glass-blur))`
    - 添加 `border-bottom: 1px solid var(--glass-border)` + `box-shadow: var(--shadow-sm)`
    - 实现 `@supports not (backdrop-filter: blur(1px))` 降级为纯色背景
    - 活跃导航项使用渐变下划线 `border-image: var(--gradient-primary) 1`
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [ ] 7.2. 升级 AppShell 组件：添加背景层次深度
    - 主内容区域添加 `background-image: var(--gradient-page-bg)` 微妙径向渐变
    - 添加 `::before` 装饰性渐变球（opacity ≤ 0.05，pointer-events: none）
    - 确保暗色模式下渐变色调自动适配（通过 CSS 变量）
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [ ]* 7.3. 编写 Property Test：视觉层次区分
    - **Property 3: Visual layer distinction**
    - 验证相邻视觉层 token 之间 HSL lightness 差异至少 1.5 个百分点
    - Tag: `Feature: ui-polish-refinement, Property 3: Visual layer distinction`
    - **Validates: Requirements 3.2**

  - [ ]* 7.4. 编写单元测试：TopNav glassmorphism 与降级
    - 测试 TopNav 应用 glassmorphism 相关 CSS 类
    - 测试 @supports 降级规则存在
    - 测试活跃导航项渐变下划线
    - _Requirements: 5.1, 5.4_

- [ ] 8. 登录页视觉升级
  - [ ] 8.1. 升级 LoginView：glassmorphism 表单 + 渐变背景
    - 全屏渐变背景（primary 色系径向渐变）
    - 登录表单卡片应用 glassmorphism 效果（glass-bg + backdrop-filter）
    - 背景区域添加品牌 SVG 装饰元素
    - 暗色模式下渐变和装饰颜色通过 CSS 变量自适应
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

  - [ ]* 8.2. 编写单元测试：登录页视觉验证
    - 测试登录页包含 glassmorphism 表单卡片
    - 测试渐变背景存在
    - 测试暗色模式适配
    - _Requirements: 8.2, 8.3, 8.4_

- [ ] 9. 页面集成与接线
  - [ ] 9.1. 在 DashboardView 中集成 StatCard + AnimatedNumber
    - 替换现有统计卡片为 StatCard 组件
    - 为数字指标接入 AnimatedNumber 组件
    - 配置 staggered delay（每张间隔 80-120ms）
    - _Requirements: 6.1, 6.3_

  - [ ] 9.2. 在各页面空状态中集成 SVG 插画
    - 在任务列表、通知、评价、课程、搜索结果等页面的 EmptyState 中传入对应 illustration
    - 确保未使用的插画不被打包（tree-shaking）
    - _Requirements: 4.1, 4.2, 9.5_

  - [ ]* 9.3. 编写单元测试：页面集成验证
    - 测试 DashboardView 渲染 StatCard 组件
    - 测试空状态页面渲染对应 SVG 插画
    - _Requirements: 4.1, 6.1_

- [ ] 10. 性能验证与最终检查
  - [ ] 10.1. 性能预算验证
    - 运行 `npm run build` 后检查 CSS 产物 gzip 大小增量 < 5KB
    - 验证 package.json 未新增运行时动画依赖
    - 验证 SVG 插画以 Vue 组件形式存在（支持 tree-shaking）
    - 验证 backdrop-filter 仅用于固定定位元素（TopNav）
    - _Requirements: 9.1, 9.2, 9.3, 9.5, 1.5_

  - [ ] 10.2. 主题一致性验证
    - 验证所有新增视觉效果在 light/dark 主题下表现正确
    - 验证主题切换时视觉属性在 200ms 内完成过渡，无闪烁
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

- [ ] 11. Final checkpoint - 全部测试通过
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- 所有 property test 使用 fast-check 库 + Vitest，每个 test 至少 100 次迭代
- 所有动效仅使用 CSS transitions/animations + Vue 内置 Transition，禁止引入 GSAP/Framer Motion/anime.js
- CSS 增量预算：< 5KB gzip

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3", "1.4", "2.1", "2.2"] },
    { "id": 2, "tasks": ["2.3", "2.4", "4.1", "4.2", "4.3"] },
    { "id": 3, "tasks": ["4.4", "5.1"] },
    { "id": 4, "tasks": ["5.2", "5.3", "7.1", "7.2"] },
    { "id": 5, "tasks": ["7.3", "7.4", "8.1"] },
    { "id": 6, "tasks": ["8.2", "9.1", "9.2"] },
    { "id": 7, "tasks": ["9.3", "10.1", "10.2"] }
  ]
}
```
