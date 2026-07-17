# QA 经验

## Bun 全局缓存中的 Playwright

- `bunx playwright` 可用但项目未安装 Playwright 时，直接导入 Bun 缓存里的 `playwright` 包可能因 Node 无法解析同级 `playwright-core` 而失败。
- QA 脚本可直接导入对应版本缓存中的 `playwright-core/index.mjs`，继续使用真实 Chrome，无需给项目增加临时依赖。

## Semi Tabs 与多选弹层

- Semi Tabs 会保留隐藏面板 DOM，相同 placeholder/text 会触发 Playwright strict mode；应先用当前 tabpanel 的 label 限定 locator 范围。
- 响应式表格调整后，隐藏 TabPane 可能仍让按文本或角色定位的模型行等待超时；可用 `.semi-table-row:visible` 限定当前可见行，并通过整行点击验证选择行为。
- Semi Select 多选后弹层保持打开，背景元素会暂时离开可访问树；当前版本在无头 Chrome 下无法通过 `Escape`、click-away 或箭头可靠关闭。需要验证后续独立状态时，应在同一登录 context 中新开 page，避免弹层污染后续证据。
- 含 portal/fixed 图层的页面不要用 `fullPage` 作为视觉证据，否则弹层可能缺失、固定导航可能被合成到长图中部；应按目标 viewport 截图并等待弹层动画稳定。

## Semi Dropdown 与 Tooltip

- 同一图标按钮同时需要 Dropdown 和 Tooltip 时，应让 Tooltip 包裹 Dropdown。反向嵌套会让点击只触发 Tooltip，Dropdown 菜单无法打开。

## 本地登录限流与浏览器 QA

- 短时间反复创建无痕浏览器并登录会触发全局 API 限流，随后静态资源也可能返回 429，表现为登录页空白或 Playwright 定位超时。
- 应在同一 browser context 中完成登录和页面验证；隔离实例已被限流时，可重启实例清空临时限流状态后一次完成 QA。
