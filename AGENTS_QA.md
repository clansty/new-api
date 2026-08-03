# QA 经验

## 大型旧 JSX 与 i18n 校验

- `EditChannelModal.jsx` 等旧文件尚未整体符合当前 Prettier，直接 `--write` 会产生数千行无关格式变化；应保持局部改动，并用构建和浏览器验证代替全文件格式化。
- `bun run i18n:extract` 会重排所有语言 JSON，新增少量文案时应精确补齐各 locale，随后用 `jq empty` 和生产构建验证，避免把纯格式噪音带入提交。
- 当前 `bun run i18n:lint` 存在大量全仓硬编码基线告警，判断本次是否新增问题时需核对告警文件和行号，不能把全量非零退出直接归因于当前改动。

## Bun 全局缓存中的 Playwright

- `bunx playwright` 可用但项目未安装 Playwright 时，直接导入 Bun 缓存里的 `playwright` 包可能因 Node 无法解析同级 `playwright-core` 而失败。
- QA 脚本可直接导入对应版本缓存中的 `playwright-core/index.mjs`，继续使用真实 Chrome，无需给项目增加临时依赖。

## Semi Tabs 与多选弹层

- Semi Tabs 会保留隐藏面板 DOM，相同 placeholder/text 会触发 Playwright strict mode；应先用当前 tabpanel 的 label 限定 locator 范围。
- Semi Tabs 中视觉上位于当前面板的工具栏按钮不一定是该 tabpanel 的可访问后代；面板内 role 定位为 0 时，可用 `button:visible` 配合精确文本筛选，并先断言数量为 1。
- 响应式表格调整后，隐藏 TabPane 可能仍让按文本或角色定位的模型行等待超时；可用 `.semi-table-row:visible` 限定当前可见行，并通过整行点击验证选择行为。
- Semi Select 多选后弹层保持打开，背景元素会暂时离开可访问树；当前版本在无头 Chrome 下无法通过 `Escape`、click-away 或箭头可靠关闭。需要验证后续独立状态时，应在同一登录 context 中新开 page，避免弹层污染后续证据。
- 含 portal/fixed 图层的页面不要用 `fullPage` 作为视觉证据，否则弹层可能缺失、固定导航可能被合成到长图中部；应按目标 viewport 截图并等待弹层动画稳定。

## Semi Dropdown 与 Tooltip

- 同一图标按钮同时需要 Dropdown 和 Tooltip 时，应让 Tooltip 包裹 Dropdown。反向嵌套会让点击只触发 Tooltip，Dropdown 菜单无法打开。
- Dropdown 菜单项打开 `Modal.confirm` 时，移动端和平板端的菜单 portal 可能覆盖确认按钮；应为 Dropdown 启用 `clickToHide`，延迟到下一事件循环再打开 Modal，并在真实窄视口截图中确认菜单已消失。

## 本地登录限流与浏览器 QA

- 短时间反复创建无痕浏览器并登录会触发全局 API 限流，随后静态资源也可能返回 429，表现为登录页空白或 Playwright 定位超时。
- 应在同一 browser context 中完成登录和页面验证；隔离实例已被限流时，可重启实例清空临时限流状态后一次完成 QA。

## 会话鉴权与 Playwright API 请求

- `/api/user/login` 返回的 session cookie 不能单独通过 `UserAuth`；Playwright context 还需要从登录响应的 `data.id` 设置 `New-Api-User` 请求头，这与前端 API helper 的行为一致。
- 登录后的页面状态依赖 `localStorage.user`，直接用 context request 登录时应把响应中的 `data` 注入 localStorage，避免 `PrivateRoute` 跳回登录页。
- 移动端 Semi `Select` 的 portal 在无头 Chromium 中可能已可交互但未进入截图合成层；对选项列表做一次 1px 预滚动再滚到目标位置，可稳定捕获顶部和底部菜单状态。

## Gin 路由尾斜杠与 curl

- 日志路由的尾斜杠并不统一：`/api/log/self/` 会重定向到 `/api/log/self`，而管理员列表使用 `/api/log/`。`curl` 管道接 `jq` 验证时应使用准确路径或加 `-L`，否则 301 HTML 会表现为 JSON 解析失败。

## Cline 中继 QA

- 新建 SQLite 实例的管理 API 除会话 Cookie 外，还需携带与会话用户 ID 一致的 `New-Api-User` 请求头。
- 未配置模型价格的隔离实例会在转发前返回 400；端到端中继 QA 可通过临时开启 `SelfUseModeEnabled` 放行未配置价格的模型。
