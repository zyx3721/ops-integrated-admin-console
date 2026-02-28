# 运维集成管理后台系统

一个面向企业内网运维场景的前后端分离系统，聚合 AD 管理、打印管理、VPN 管理，并提供统一登录、项目凭据隔离、异步执行进度与操作审计能力。

## 项目特性

- 前后端分离：`Go + SQLite3` 后端，`Vue3 + Vite + Naive UI` 前端
- 统一管理员认证：支持注册、登录、修改密码
- 多项目聚合：AD / 打印 / VPN 功能统一入口
- 用户级凭据隔离：每个管理员独立维护项目账号密码
- 凭据加密存储：数据库中项目密码按密钥加密
- 异步执行机制：任务提交后轮询进度，支持日志逐条输出
- 可审计日志：登录、项目加载、项目操作全链路记录
- 缓存倒计时与自动重登：避免会话长期失效造成突发报错

## 技术栈

### 后端

- Go（以 `backend/go.mod` 为准）
- SQLite3（`modernc.org/sqlite`）
- `golang.org/x/crypto`
- `github.com/xuri/excelize/v2`

### 前端

- Vue 3
- Vite
- TypeScript
- Pinia
- Vue Router
- Naive UI

## 目录结构

```text
ops-integrated-admin-console
├─ backend
│  ├─ main.go
│  ├─ .env.example
│  ├─ .env              # 本地复制生成（不提交）
│  ├─ go.mod / go.sum
│  ├─ db
│  │  └─ ops_admin.db
│  ├─ data
│  │  └─ ad
│  │     ├─ templates
│  │     │  └─ 创建AD用户模板.xlsx
│  │     └─ uploads
│  └─ internal
│     ├─ runtime
│     │  ├─ bootstrap.go
│     │  ├─ config.go
│     │  ├─ db.go
│     │  ├─ handlers.go
│     │  ├─ async_jobs.go
│     │  └─ project_bridge.go
│     └─ project
│        ├─ common.go
│        ├─ ad.go
│        ├─ print.go
│        └─ vpn.go
├─ frontend
│  ├─ .env
│  ├─ index.html
│  ├─ public
│  │  └─ favicon.ico
│  └─ src
└─ README.md
```

## 功能清单

### 认证与账号

- 管理员注册
- 管理员登录
- 登录态校验（Token + 过期时间）
- 修改管理员密码
- 数据库初始化时自动创建默认管理员：`admin / admin123`

### 项目凭据

- 项目类型：`ad`、`print`、`vpn`、`vpn_firewall`
- 每个管理员独立配置并隔离
- 保存凭据后重置该项目加载状态，下一次进入会重新校验登录
- 凭据密码字段加密存储（`enc:v1:` 前缀）

### AD 管理

- 新增用户
- 批量新增用户（模板下载 + 上传执行 + 结果表格）
- 查询用户
- 重置密码
- 解锁用户
- 修改姓名
- 修改描述
- 删除用户

### 打印管理

- 新增用户
- 查询用户
- 重置密码
- 修改用户（查询确认后进入编辑态）
- 删除用户

### VPN 管理

- 新增用户
- 查询用户
- 修改密码
- 修改状态
- 删除用户
- 可选同步删除防火墙上的 VPN 账户

### 操作日志

- 记录登录、注册、项目加载、项目操作成功/失败
- 支持分页查询
- 支持每页条数：`20 / 30 / 50 / 100 / 200`

## 运行机制

### 项目加载机制

- 首次点击 AD/打印/VPN 菜单时，触发 `/api/projects/{project}/load`
- 后端使用当前管理员保存的项目凭据执行登录验证
- 校验通过后写入项目加载状态缓存
- 同一会话下再次进入该项目可直接使用

### 缓存倒计时机制

- 倒计时时长由后端 `PROJECT_CACHE_TTL_MINUTES` 控制
- 前端页面右上角显示剩余时间
- 倒计时归零时，前端静默调用 `/api/projects/relogin` 重登已配置项目
- 输入框聚焦、项目加载中、任务执行中时会暂停倒计时

### 异步执行机制

- 前端调用 `/api/projects/operate-async` 创建任务
- 后端返回 `job_id`
- 前端轮询 `/api/projects/operate-async/{job_id}` 获取状态
- 支持进度百分比、日志增量、结果文本、结果项列表
- 任务完成后前端根据动作重置表单或保留结果

## 环境变量

后端环境变量示例文件：`backend/.env.example`  
后端实际环境变量文件：`backend/.env`（由示例复制后填写）

```env
# 后端服务配置（已脱敏示例，发布到公网仓库可直接保留）
AD_API_URL=http://ad.example.internal/
PRINT_API_URL=http://print.example.internal/printhub/
VPN_SSH_ADDR=vpn.example.internal
FIREWALL_SSH_ADDR=firewall.example.internal

# 后端运行参数
ADDR=:8080
PROJECT_CACHE_TTL_MINUTES=15

# 凭据加密主密钥（建议不少于 16 位，生产环境请替换）
CREDENTIAL_SECRET=change-this-to-your-own-secret-key

# 可选：历史密钥回退列表（用于密钥轮换兼容，多个用英文逗号分隔）
CREDENTIAL_SECRET_FALLBACKS=change-me-ops-credential-secret
```

前端环境变量文件：`frontend/.env`

```env
VITE_API_BASE=http://127.0.0.1:8080
```

## 环境要求

建议先确认本机运行环境，再执行安装与启动。

| 组件 | 最低要求 | 推荐版本 | 说明 |
| --- | --- | --- | --- |
| Go | 1.26+ | 1.26.x | 后端 `go.mod` 当前声明为 `go 1.26` |
| Node.js | 18+ | 20 LTS | 用于前端开发与构建 |
| npm | 9+ | 10+ | 与 Node.js 版本配套使用 |
| Git | 2.30+ | 最新稳定版 | 用于代码拉取与发布 |

版本检查命令：

```bash
go version
node -v
npm -v
git --version
```

## 快速开始

### 1. 拉取代码

```bash
git clone https://github.com/zyx3721/ops-integrated-admin-console.git
cd ops-integrated-admin-console
```

### 2. 初始化环境变量

```bash
cp backend/.env.example backend/.env
```

复制后按你的实际环境修改 `backend/.env`（内网地址、密钥等）。

### 3. 启动后端

```bash
cd backend
go mod tidy
go run .
```

### 4. 启动前端

```bash
cd ../frontend
npm install
npm run dev
```

### 5. 访问系统

- 前端默认开发地址：`http://127.0.0.1:5173`
- 后端健康检查：`http://127.0.0.1:8080/healthz`

## 数据库说明

- 固定路径：`backend/db/ops_admin.db`
- 数据库不存在时会自动初始化表结构
- 管理员表为空时自动创建默认管理员 `admin/admin123`
- 主要表：
- `admins`
- `auth_tokens`
- `project_credentials`
- `project_load_state`
- `operation_logs`

## 常见问题

### 1. `database is locked (SQLITE_BUSY)`

- 原因：多个后端实例同时占用同一个 SQLite 文件
- 处理：结束重复进程，确保仅保留一个后端实例

### 2. `decrypt credential failed`

- 原因：`CREDENTIAL_SECRET` 已更换，旧密文无法解密
- 处理：
- 将旧密钥追加到 `CREDENTIAL_SECRET_FALLBACKS`
- 或重新保存一次项目凭据

### 3. 登录后提示“项目凭据未配置”

- 原因：当前管理员未配置对应项目账号密码
- 处理：先到“项目凭据”页面保存，再进入项目管理

### 4. 前端中文显示乱码

- 原因：文件编码不是 UTF-8 或终端/编辑器使用了错误编码
- 处理：统一使用 UTF-8 编码保存与查看

## 安全建议

- 不要将真实生产账号密码提交到 Git
- 发布前建议再次检查 `backend/.env`、`frontend/.env`
- `CREDENTIAL_SECRET` 使用高强度随机字符串并妥善保管
- 定期备份 `backend/db/ops_admin.db`

## 构建命令

后端构建：

```bash
cd ops-integrated-admin-console/backend
go build ./...
```

前端构建：

```bash
cd ../frontend
npm run build
```

## 致谢

- 前端基础模板参考：`mars-admin`
- 原业务逻辑参考：`AD_Tool`、`Print_Tool`、`VPN_Tool`（用于迁移对照）


