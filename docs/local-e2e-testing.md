# 本地端到端（E2E）与测试经验

本文记录在 new-api 仓库做本地端到端验证的标准流程与踩过的坑，便于后续开发/改动（尤其是涉及前后端联动、路由、鉴权的改动）快速自测。

## 1. 整体流程（一次完整的本地 E2E）

```bash
# 1) 先构建前端（关键：前端是编译期嵌入二进制的，必须先 build）
cd web && bun run build && cd ..

# 2) 再构建后端二进制（会通过 //go:embed web/dist 把刚构建的前端打包进去）
go build -o /tmp/new-api-build .

# 3) 用「一次性数据库 + 独立端口」启动，避免污染真实数据 / 端口冲突
rm -f /tmp/newapi-pw.db
GIN_MODE=release PORT=3111 \
  SQLITE_PATH='/tmp/newapi-pw.db?_busy_timeout=30000' \
  /tmp/new-api-build

# 4) 空库首次启动需要初始化 root 管理员
curl -s -X POST http://localhost:3111/api/setup \
  -H 'Content-Type: application/json' \
  -d '{"username":"root","password":"rootroot123","confirmPassword":"rootroot123"}'

# 5) 用 Playwright 打开 http://localhost:3111 登录并验证
```

> 真实数据库 `one-api.db` 不要直接拿来测试。需要真实数据时，**复制一份**到 `/tmp` 再用 `SQLITE_PATH` 指过去，启动时的自动迁移不会动到原库。

## 2. 必须牢记的坑（按重要度）

### 2.1 前端是编译期嵌入 —— 改完前端必须重新 `bun run build` 再 `go build`
`main.go` 里 `//go:embed web/dist`。只改前端源码、不重新构建前端 + 后端，跑出来的还是旧 UI。
**顺序固定为：前端 build → 后端 build → 跑二进制。** 只改 Go 代码时可跳过前端 build。

### 2.2 gin 路由冲突在「启动时 panic」，不是编译期
新增路由后，光 `go build` 通过不代表没问题。务必**真正启动一次**：
- 用 `GIN_MODE=debug` 启动，会打印完整路由表，确认新路由确实注册成功。
- 冒烟测试：未登录访问受保护接口，**401 = 路由存在且有鉴权**，**404 = 路由没注册**。

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3111/api/user/1/tokens/   # 期望 401
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3111/api/user/1/nonexist  # 期望 404
```

gin 支持「同层静态段 + 路径参数」共存（如 `/tokens/search` 与 `/tokens/:token_id`、`/tokens/batch` 与 `/tokens/:token_id/key`），新增这类路由不会冲突。

### 2.3 浏览器会缓存 GET 的 404 —— 加完路由要清缓存
这次 E2E 抓到的真实 bug 就栽在这：管理员编辑令牌时前端会 `GET /api/user/:id/tokens/:token_id` 加载详情，但**该路由当初漏注册**。补好路由、重启服务后，浏览器里仍然 404 —— 因为它把**修复前那次 404 响应缓存了**，直接走缓存不打服务器。

排查手法：用 `fetch(url, { cache: 'no-store' })` 在浏览器里直连接口，若返回 200 而 UI 仍 404，基本就是缓存。处理：
```js
const cdp = await page.context().newCDPSession(page);
await cdp.send('Network.clearBrowserCache');
```
> 同时也是一个提醒：新接口最好从一开始就设计对，否则「先 404 后修好」的缓存假象会浪费排查时间。

### 2.4 鉴权需要 `New-Api-User` 头
用 raw `fetch`/`curl` 直连业务接口时，除了 session cookie，还要带 `New-Api-User: <用户id>` 头，否则 `authHelper` 直接 401。浏览器里前端的 axios 封装会自动加，手写请求要自己补：
```js
const uid = JSON.parse(localStorage.getItem('user')).id;
await fetch('/api/user/self/groups', { headers: { 'New-Api-User': String(uid) } });
```

### 2.5 没配 `SESSION_SECRET` 时，重启服务会让旧会话失效
session secret 每次启动随机生成，**重启二进制后浏览器需要重新登录**。多次重启调试时别困惑于「突然 401」。

## 3. Playwright 操作 Semi Design UI 的要点

- **复选框（Checkbox）**：真正的 `<input>` 被 `.semi-checkbox-inner-display` 覆盖，直接点 input 会报 "intercepts pointer events"。应点 `.semi-checkbox`（label 容器），React 的 onChange 才会触发：
  ```js
  await row.locator('.semi-checkbox').nth(1).click();
  ```
- **下拉框（Select）**：选项渲染在 body 下的**浮层** `.semi-select-option-list`；若用了自定义 `renderOptionItem`，可能没有 `.semi-select-option` 这个类，直接读浮层 `innerText` 更稳：
  ```js
  await sel.click();
  const text = await page.locator('.semi-select-option-list').innerText();
  ```
- **SideSheet/弹层比视口宽**：按钮会「outside of viewport」点不到。先 `browser_resize` 放大（如 1680×1000），或对按钮用 `click({ force: true })`。
- **用 Network 面板交叉验证**：行为对不一定接口对。过滤 `/api/...` 确认请求确实打到了**期望的端点**（例如管理员代管要看到 `/api/user/:id/tokens`，而不是自助的 `/api/token`）。

## 4. 改动后的「验证清单」（本次实际执行）

后端：
- [x] `go build` 通过（编译）
- [x] `GIN_MODE=debug` 启动，路由表里能看到新接口
- [x] 未登录冒烟：受保护接口 401、不存在接口 404

前端：
- [x] `bunx eslint <改动文件>` 通过（注意：个别历史组件缺 license header 是既有问题，与本次改动无关）
- [x] `bunx prettier --list-different <改动文件>` 确认新代码符合格式
- [x] `bun run build` 整体构建通过

端到端（Playwright，登录真实跑一遍）：
- [x] UI 元素确实渲染（新增的列/菜单/弹层）
- [x] 关键交互成功并有成功 toast
- [x] Network 面板确认打到正确端点、状态码正确
- [x] 覆盖增 / 删 / 改 / 查全链路，而不仅是「打开能看到」

## 5. 一个容易自伤的格式化坑

对**历史上本就不符合 prettier 的文件**直接 `prettier --write`，会把大量无关行一起重排，污染本次提交 diff（实测一个文件被改了 300+ 行）。做法：
- 只对**自己新建的文件**整体 `--write`；
- 对**改动的存量文件**，用 `--list-different` / `diff` 确认差异是否落在自己改的行上，是历史遗留的就**不要顺手格式化**，保持小而聚焦的 diff。
