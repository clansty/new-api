# QA 经验

## zsh 路由批量请求

- zsh 的 `path` 是与 `PATH` 绑定的特殊数组，批量 curl 脚本中给 `path` 赋值会覆盖命令搜索路径，表现为循环首轮开始后 `curl: command not found`；路由变量应使用 `request_path` 等非保留名称。

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

本次子 key QA 再次复现该限制：多次独立 Playwright context 登录后，静态 `/console` 页面会返回 429；重启隔离后端并复用单一 context 可恢复。

## 会话鉴权与 Playwright API 请求

- `/api/user/login` 返回的 session cookie 不能单独通过 `UserAuth`；Playwright context 还需要从登录响应的 `data.id` 设置 `New-Api-User` 请求头，这与前端 API helper 的行为一致。
- 登录后的页面状态依赖 `localStorage.user`，直接用 context request 登录时应把响应中的 `data` 注入 localStorage，避免 `PrivateRoute` 跳回登录页。
- 移动端 Semi `Select` 的 portal 在无头 Chromium 中可能已可交互但未进入截图合成层；对选项列表做一次 1px 预滚动再滚到目标位置，可稳定捕获顶部和底部菜单状态。

## Gin 路由尾斜杠与 curl

- 日志路由的尾斜杠并不统一：`/api/log/self/` 会重定向到 `/api/log/self`，而管理员列表使用 `/api/log/`。`curl` 管道接 `jq` 验证时应使用准确路径或加 `-L`，否则 301 HTML 会表现为 JSON 解析失败。

## Cline 中继 QA

- 新建 SQLite 实例的管理 API 除会话 Cookie 外，还需携带与会话用户 ID 一致的 `New-Api-User` 请求头。
- 未配置模型价格的隔离实例会在转发前返回 400；端到端中继 QA 可通过临时开启 `SelfUseModeEnabled` 放行未配置价格的模型。

## 隔离后端进程与慢上游 QA

- 通过一次性 shell 后台命令启动隔离实例时，命令执行器可能在父 shell 结束后回收子进程；需要后续多次 `curl` 的 QA 应使用持久 PTY 会话，并在完成后发送 SIGINT 验证优雅退出。
- 用 `nc` 模拟首响应超时上游时，延迟响应的计时从监听进程启动就开始；应预留足够长的延迟并立即发起请求，否则响应可能在客户端连接前已进入管道缓冲，导致请求看似瞬时成功。

## 渠道组弹窗 QA

- Codex 内置浏览器不可用时，可按 browser skill 的降级路径使用独立 Playwright 脚本；Semi 弹窗动画结束前定位 footer 按钮容易命中旧 DOM，应等待过渡稳定并限定到当前可见弹窗。
- 成员表仅依赖横向滚动会让平板和手机端的右侧操作不可发现；窄屏应保留固定操作列，将完整命令收进菜单，并截图验证菜单和删除确认均可触达。
- 使用 `useIsMobile` 的组件需要在目标宽度重新加载页面，避免响应式状态尚未更新时截到桌面列配置。

## 渠道内存缓存 QA

- 调用 `InitChannelCache()` 的测试夹具必须为启用渠道同时创建匹配 `Group`、`Models` 的 `Ability`；只有渠道而没有能力记录会让分组模型映射缺失，导致缓存初始化 panic。

## 响应式日志展开行 QA

- Semi Table 的单元格 class 包含 `semi-table-row-cell`，用 `contains(@class, "semi-table-row")` 会误定位到 `<td>`；Playwright 应精确匹配 class token 或直接定位 `<tr>`，再点击首列展开控件。
- 移动端 `CardTable` 的展开内容位于页面内部滚动容器，`document.body.scrollHeight` 不会随详情增长；截图前应对目标详情调用 `scrollIntoViewIfNeeded()`，再检查目标矩形和横向 scroll width。
- `Descriptions` 的移动端值列较窄，嵌套详情应使用纵向布局和 `overflowWrap: anywhere`，避免横排标签截断渠道名、成员名等诊断信息。
