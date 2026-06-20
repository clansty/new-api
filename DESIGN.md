# new-api Design System

## 1. Atmosphere & Identity

new-api 的管理台是一个高密度、偏运维的控制面板。界面应保持安静、清晰、可快速扫描；主要视觉签名是 Semi Design 的中性表面、圆形状态标签和紧凑表单，而不是营销式装饰。

## 2. Color

### Palette

| Role | Token | Light | Dark | Usage |
|------|-------|-------|------|-------|
| Surface/primary | --semi-color-bg-0 | Semi default | Semi default | 页面和 SideSheet 背景 |
| Surface/elevated | --semi-color-bg-2 | Semi default | Semi default | Card、Modal、Popover |
| Text/primary | --semi-color-text-0 | Semi default | Semi default | 表单标签、正文、标题 |
| Text/secondary | --semi-color-text-2 | Semi default | Semi default | 帮助说明、弱提示 |
| Border/default | --semi-color-border | Semi default | Semi default | 表格、表单、分隔线 |
| Accent/primary | --semi-color-primary | Semi default | Semi default | 主按钮、可操作入口 |
| Status/success | Semi green | Semi green | Semi green | 启用、成功状态 |
| Status/warning | Semi yellow | Semi yellow | Semi yellow | 管理员、提醒状态 |
| Status/error | Semi red | Semi red | Semi red | 删除、禁用、错误 |
| Status/info | Semi blue | Semi blue | Semi blue | 普通信息、编辑状态 |

### Rules

- 优先使用 Semi UI 的语义 token 和组件色，不新增原始 hex 色值。
- 状态含义用 Semi `Tag` 颜色表达；按钮只在明确可操作时使用主色。
- 管理台页面避免渐变、装饰背景和大面积单色块。

## 3. Typography

### Scale

| Level | Size | Weight | Line Height | Tracking | Usage |
|-------|------|--------|-------------|----------|-------|
| H2 | 20px | 600 | 1.4 | 0 | SideSheet 标题 |
| H3 | 18px | 500 | 1.4 | 0 | 卡片标题 |
| Body | 14px | 400 | 1.5 | 0 | 表单、表格、正文 |
| Caption | 12px | 400 | 1.4 | 0 | 辅助说明、元数据 |

### Font Stack

- Primary: Semi UI default system sans-serif
- Mono: browser/system monospace where code-like labels are required

### Rules

- 表单和表格正文保持 14px；辅助说明可用 12px。
- 不使用 hero 级大字；管理页标题保持紧凑。
- 字距保持 0。

## 4. Spacing & Layout

### Base Unit

All spacing derives from a base of **4px**.

| Token | Value | Usage |
|-------|-------|-------|
| --space-1 | 4px | 图标与文字、紧凑间距 |
| --space-2 | 8px | 表单块内默认间距 |
| --space-3 | 12px | Row gutter、分组间距 |
| --space-4 | 16px | 卡片内边距 |
| --space-6 | 24px | 弹窗主区域间距 |

### Grid

- 管理表单使用 Semi `Row` / `Col` 的 24 栅格。
- 桌面 SideSheet 宽度 600px；移动端 100%。
- 表单项优先单列，密集设置可用 10/14 或 12/12 分栏。

### Rules

- 使用 Tailwind 间距工具时保持 4px 倍数。
- 不嵌套装饰卡片；SideSheet 内的 Card 只承载明确的设置分组。

## 5. Components

### Admin SideSheet Form

- **Structure**: `SideSheet` footer 操作区 + `Spin` + `Form` + 分组 `Card`。
- **Variants**: 新建、编辑。
- **Spacing**: 表单区域 `p-2`，分组间距 `space-y-3`。
- **States**: loading、submit、cancel、validation error。
- **Accessibility**: 使用 Semi 表单标签和原生按钮语义。
- **Motion**: 使用 Semi 默认 SideSheet/Modal 动效。

### Setting Switch

- **Structure**: `Form.Switch` + label + `extraText`。
- **Variants**: 开/关。
- **Spacing**: 占满一行，避免和数字输入挤在同一行。
- **States**: checked、unchecked、disabled、focus。
- **Accessibility**: label 必须直接说明开关影响。
- **Motion**: 使用 Semi 默认 toggle 动效。

## 6. Motion & Interaction

### Timing

| Type | Duration | Easing | Usage |
|------|----------|--------|-------|
| Micro | Semi default | Semi default | Button、Switch |
| Standard | Semi default | Semi default | Modal、SideSheet |

### Rules

- 沿用 Semi UI 默认动效。
- 不手写 layout 动画。
- 所有可点击图标按钮必须有清晰 label 或可见文本。

## 7. Depth & Surface

### Strategy

mixed

- 弹窗、SideSheet、Popover 使用 Semi 默认层级。
- 表单分组可使用轻量 Card，但不在 Card 内再嵌套 Card。
- 阴影仅沿用既有 `shadow-sm` / Semi 默认层级，不新增重阴影。
