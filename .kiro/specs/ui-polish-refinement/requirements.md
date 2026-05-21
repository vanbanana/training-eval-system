# Requirements Document

## Introduction

对现有智能实训评价管理系统前端进行 UI 精致化改造，将界面从"功能完整但粗糙的后台管理系统"提升到现代 B 端 SaaS 产品的设计水准。改造聚焦于视觉层次感、质感和精致度，运用 subtle glassmorphism、soft shadows、layered depth、gradient accents 等 2025 年 B 端设计趋势，同时严格控制性能开销。

技术约束：Vue 3 + Vite + shadcn-vue + Tailwind CSS，不引入重型动画库，不改变现有功能逻辑。

## Glossary

- **UI_System**: 前端界面系统，包含所有 Vue 组件、全局样式、设计 token 的集合
- **Design_Token_Layer**: globals.css 中定义的 CSS 变量体系，包含颜色、圆角、阴影、间距等设计原子
- **Card_Component**: 系统中用于承载内容块的卡片容器组件（shadcn-vue Card 及自定义卡片样式）
- **AppShell**: 全局布局壳组件，包含 TopNav 和主内容区域
- **Empty_State_Component**: 页面无数据时展示的空状态组件
- **Stat_Card**: Dashboard 中展示关键指标的统计卡片
- **SVG_Illustration**: 用于提升界面精致度的立体感 SVG 插画资源
- **Glassmorphism_Effect**: 基于 backdrop-filter 的毛玻璃视觉效果
- **Soft_Shadow**: 使用多层、大扩散半径、低透明度的柔和阴影效果
- **Performance_Budget**: 改造不得增加的性能开销上限（无新 JS 运行时依赖、CSS 文件增量 < 5KB gzip）

## Requirements

### Requirement 1: 扩展设计 Token 体系

**User Story:** As a 前端开发者, I want 设计 token 体系包含阴影、渐变、毛玻璃等高级视觉属性, so that 所有页面可以通过统一变量获得一致的精致视觉效果。

#### Acceptance Criteria

1. THE Design_Token_Layer SHALL 定义至少 3 级柔和阴影变量（shadow-sm、shadow-md、shadow-lg），每级使用多层 box-shadow 且最大透明度不超过 0.08
2. THE Design_Token_Layer SHALL 定义 glassmorphism 相关变量，包含 backdrop-blur 值（8px-16px）和半透明背景色（白色 alpha 0.6-0.8）
3. THE Design_Token_Layer SHALL 定义至少 2 组渐变 token（用于卡片背景点缀和按钮高亮状态）
4. THE Design_Token_Layer SHALL 同时为 light 和 dark 主题提供对应的阴影、毛玻璃、渐变变量值
5. WHEN 新增 token 被编译后, THE Design_Token_Layer SHALL 使 CSS 文件增量不超过 5KB（gzip 后）

### Requirement 2: 卡片与容器视觉升级

**User Story:** As a 系统用户, I want 所有卡片和内容容器具有柔和阴影和微妙的层次感, so that 界面不再扁平单调，具备现代 SaaS 产品的质感。

#### Acceptance Criteria

1. THE Card_Component SHALL 默认应用 shadow-sm 级别的柔和阴影，替代当前纯 border 样式
2. WHEN 用户将鼠标悬停在 Card_Component 上时, THE Card_Component SHALL 在 200ms 内过渡到 shadow-md 级别阴影
3. THE Stat_Card SHALL 在左侧或顶部展示 4px 宽的渐变色条作为视觉锚点，颜色与卡片语义对应（如成功为绿色渐变、警告为橙色渐变）
4. THE Card_Component SHALL 使用 border-radius 为 radius-lg（10px）以上的圆角值
5. THE Card_Component SHALL 保持 border 为 1px solid border 色，阴影作为叠加层而非替代

### Requirement 3: 页面背景与层次深度

**User Story:** As a 系统用户, I want 页面背景具有微妙的层次变化, so that 内容区域与背景之间有清晰的视觉分离感。

#### Acceptance Criteria

1. THE AppShell SHALL 在主内容区域背景上应用微妙的径向渐变或噪点纹理，增加视觉深度
2. THE UI_System SHALL 确保内容卡片（surface 层）与页面背景（background 层）之间存在至少 2 级视觉层次区分
3. WHILE 暗色主题激活时, THE AppShell SHALL 使用深色渐变背景替代纯色背景，保持层次感一致
4. THE AppShell SHALL 使背景装饰元素（渐变/纹理）的 opacity 不超过 0.05，避免干扰内容可读性

### Requirement 4: 空状态与插画系统

**User Story:** As a 系统用户, I want 空状态页面展示精致的 SVG 插画, so that 无数据时界面依然美观且具有引导性。

#### Acceptance Criteria

1. THE Empty_State_Component SHALL 支持接收 SVG 插画组件作为 illustration prop，替代当前的纯图标展示
2. THE UI_System SHALL 为至少 5 个核心空状态场景提供立体感 SVG 插画（无任务、无通知、无评价、无课程、搜索无结果）
3. WHEN SVG_Illustration 被渲染时, THE SVG_Illustration SHALL 使用 CSS 变量引用当前主题色，确保在 light/dark 主题下颜色自适应
4. THE SVG_Illustration SHALL 每个文件体积不超过 8KB（未压缩），且不使用外部图片引用
5. THE Empty_State_Component SHALL 在插画下方保持标题、描述文案和操作按钮的布局结构

### Requirement 5: 导航栏精致化

**User Story:** As a 系统用户, I want 顶部导航栏具有精致的毛玻璃效果和层次感, so that 导航区域在滚动时依然清晰可辨且视觉高级。

#### Acceptance Criteria

1. THE TopNav SHALL 应用 Glassmorphism_Effect，使用 backdrop-blur 和半透明背景色
2. WHEN 页面内容滚动到导航栏下方时, THE TopNav SHALL 保持毛玻璃效果使下方内容可见但不干扰导航可读性
3. THE TopNav SHALL 在导航项激活状态下使用渐变下划线或柔和高亮背景替代当前纯色 border-bottom
4. THE TopNav SHALL 确保毛玻璃效果在不支持 backdrop-filter 的浏览器中优雅降级为纯色背景

### Requirement 6: 统计卡片动效与微交互

**User Story:** As a 系统用户, I want Dashboard 统计卡片具有入场动效和数字滚动效果, so that 数据展示更加生动且具有现代感。

#### Acceptance Criteria

1. WHEN Dashboard 页面加载完成时, THE Stat_Card SHALL 以 staggered 方式（每张间隔 80-120ms）从下方淡入
2. THE Stat_Card SHALL 使用 CSS @starting-style 或 Vue Transition 实现入场动效，不引入第三方动画库
3. WHEN 统计数字从 0 变化到目标值时, THE AnimatedNumber 组件 SHALL 在 600-800ms 内完成数字滚动动画
4. THE Stat_Card SHALL 在 hover 时展示微妙的 translateY(-2px) 上浮效果，配合阴影加深

### Requirement 7: 按钮与交互元素精致化

**User Story:** As a 系统用户, I want 按钮和交互元素具有精致的视觉反馈, so that 每次点击和悬停都感觉流畅且有质感。

#### Acceptance Criteria

1. THE UI_System SHALL 为 primary 按钮添加微妙的内阴影（inset shadow）和顶部 1px 高光线，模拟立体按压感
2. WHEN 用户悬停在 primary 按钮上时, THE UI_System SHALL 在 150ms 内展示亮度提升和阴影扩大的过渡效果
3. WHEN 用户点击按钮时, THE UI_System SHALL 展示 scale(0.97) 的按压反馈，持续 100ms
4. THE UI_System SHALL 为所有可交互元素（按钮、链接、卡片）统一使用 transition duration 150-200ms 和 ease-out 缓动函数

### Requirement 8: 登录页视觉升级

**User Story:** As a 系统用户, I want 登录页具有现代感的视觉设计, so that 第一印象即传达产品的专业性和精致度。

#### Acceptance Criteria

1. THE UI_System SHALL 在登录页左侧或背景区域展示品牌相关的大尺寸 SVG 插画或渐变装饰
2. THE UI_System SHALL 为登录表单卡片应用 Glassmorphism_Effect，使其悬浮于背景之上
3. THE UI_System SHALL 在登录页背景使用柔和的渐变色（基于 primary 色系），替代当前纯色背景
4. WHILE 暗色主题激活时, THE UI_System SHALL 调整登录页渐变和插画颜色以适配暗色环境

### Requirement 9: 性能约束保障

**User Story:** As a 前端开发者, I want UI 精致化改造不增加运行时性能负担, so that 系统在低配硬件上依然流畅运行。

#### Acceptance Criteria

1. THE UI_System SHALL 不引入任何新的 JavaScript 运行时动画库（如 GSAP、Framer Motion、anime.js）
2. THE UI_System SHALL 确保所有动效仅使用 CSS transitions、CSS animations 或 Vue 内置 Transition 组件实现
3. THE UI_System SHALL 确保 backdrop-filter 仅应用于固定定位元素（如 TopNav），避免在滚动列表中使用
4. WHEN 页面首次加载时, THE UI_System SHALL 确保 Largest Contentful Paint (LCP) 增量不超过 100ms
5. THE UI_System SHALL 确保所有 SVG 插画以内联组件形式引入，支持 tree-shaking，未使用的插画不打包

### Requirement 10: 主题一致性与暗色模式

**User Story:** As a 系统用户, I want 所有精致化效果在 light 和 dark 主题下表现一致, so that 切换主题时视觉体验不降级。

#### Acceptance Criteria

1. THE UI_System SHALL 确保所有新增视觉效果（阴影、毛玻璃、渐变）通过 CSS 变量控制，支持主题切换时自动适配
2. WHILE 暗色主题激活时, THE UI_System SHALL 将阴影颜色从黑色系调整为更深的背景色系，避免"悬浮黑洞"效果
3. WHILE 暗色主题激活时, THE UI_System SHALL 将毛玻璃背景色从白色半透明调整为深色半透明
4. THE UI_System SHALL 确保主题切换时所有视觉属性在 200ms 内完成过渡，无闪烁
