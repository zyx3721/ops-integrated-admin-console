# 运维集成管理后台系统

一个面向企业内网运维场景的前后端分离系统，整合 AD、打印、VPN 三类项目管理能力，提供统一登录、项目凭据隔离、项目会话复用、异步任务执行、缓存倒计时重登与操作日志审计。

# 一、项目特性

- 前后端分离：`Go + SQLite3` 后端，`Vue3 + Vite + Naive UI` 前端
- 统一管理员认证：支持注册、登录、修改密码
- 多项目聚合：AD / 打印 / VPN 功能统一入口
- 用户级凭据隔离：每个管理员独立维护项目账号密码
- 凭据加密存储：数据库中项目密码按密钥加密
- 项目会话复用：首次进入项目后建立会话，后续操作默认复用，不会每次操作都重新登录
- 异步执行机制：任务提交后轮询进度，支持日志逐条输出
- 可审计日志：登录、项目加载、项目操作全链路记录
- 缓存倒计时与自动重登：避免会话长期失效造成突发报错
- 多窗口倒计时同步：同一浏览器同一 Token 下共享项目缓存倒计时
- 页面关闭超时控制：页面关闭超过设定时长后重新访问需重新登录，并联动清理后端 Token 与项目会话
- 会话状态可视化：页面可直接看到项目当前处于首次登录、复用会话还是倒计时重登

# 二、技术栈

## 2.1 后端

- Go（以 `backend/go.mod` 为准）
- SQLite3（`modernc.org/sqlite`）
- `golang.org/x/crypto`
- `github.com/xuri/excelize/v2`

## 2.2 前端

- Vue 3
- Vite
- TypeScript
- Pinia
- Vue Router
- Naive UI

## 2.3 目录结构

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
│     │  ├─ common.go
│     │  ├─ config.go
│     │  ├─ db.go
│     │  ├─ handlers.go
│     │  ├─ async_jobs.go
│     │  ├─ auth_sessions.go
│     │  ├─ project_bridge.go
│     │  └─ session_manager.go
│     └─ project
│        ├─ common.go
│        ├─ ad.go
│        ├─ print.go
│        ├─ session.go
│        └─ vpn.go
├─ frontend
│  ├─ index.html
│  ├─ package.json
│  ├─ vite.config.ts
│  ├─ public
│  │  ├─ favicon.ico
│  │  └─ vite.svg
│  └─ src
│     ├─ api
│     ├─ components
│     ├─ config
│     ├─ layout
│     ├─ router
│     ├─ stores
│     ├─ styles
│     ├─ utils
│     ├─ views
│     ├─ App.vue
│     └─ main.ts
└─ README.md
```

# 三、功能清单

## 3.1 认证与账号

- 管理员注册
- 管理员登录
- 登录态校验（Token + 过期时间）
- 修改管理员密码
- 首次使用需先注册管理员账号（系统不再内置默认账号）

## 3.2 项目凭据

- 项目类型：`ad`、`print`、`vpn`、`vpn_firewall`
- 每个管理员独立配置并隔离
- 保存凭据后会清理该项目当前会话，下一次进入或执行操作时会重新校验登录
- 凭据密码字段加密存储（`enc:v1:` 前缀）

## 3.3 AD 管理

- 新增用户
- 批量新增用户（模板下载 + 上传执行 + 结果表格）
- 查询用户
- 重置密码
- 解锁用户
- 修改姓名
- 修改描述
- 删除用户

## 3.4 打印管理

- 新增用户
- 查询用户
- 重置密码
- 修改用户（查询确认后进入编辑态）
- 删除用户

## 3.5 VPN 管理

- 新增用户
- 查询用户
- 修改密码
- 修改状态
- 删除用户
- 可选同步删除防火墙上的 VPN 账户

## 3.6 操作日志

- 记录登录、注册、项目加载、项目操作成功/失败
- 支持分页查询
- 支持每页条数：`20 / 30 / 50 / 100 / 200`

## 3.7 会话状态可视化

- 项目页面提供会话状态日志，便于观察当前项目会话是否已建立或被重登
- 支持展示 `首次登录 / 复用会话 / 倒计时重登` 三种状态
- 保存项目凭据后会记录“项目会话已清理”，提示下次进入项目时重新登录

# 四、运行机制

## 4.1 项目加载机制

- 首次点击 AD/打印/VPN 菜单时，触发 `/api/projects/{project}/load`
- 后端使用当前管理员保存的项目凭据执行登录验证
- 校验通过后在当前系统 Token 下建立该项目会话缓存
- 同一 Token 下再次进入该项目或执行项目操作时，会优先复用现有会话，而不是每次操作都重新登录
- 当项目凭据被修改，或缓存倒计时到期后触发重登时，会先清理旧会话再重新建立

## 4.2 缓存倒计时机制

- 倒计时时长由后端 `PROJECT_CACHE_TTL_MINUTES` 控制
- 前端页面右上角显示剩余时间
- 倒计时以“当前系统 Token 对应的项目会话”作为作用范围
- 同一浏览器下如果同账号同 Token 打开多个窗口或标签页，会通过本地存储共享同一倒计时
- 倒计时归零时，前端静默调用 `/api/projects/relogin`，清理当前 Token 下的项目会话并重新登录已使用过的项目
- 项目重登录接口会返回 `session_state=countdown_relogin`，前端可据此展示“倒计时重登”状态日志
- 前端使用本地锁避免多个窗口在同一时刻重复触发重登
- 输入框聚焦、项目加载中、任务执行中时会暂停倒计时
- 同一浏览器下如果打开了多个系统页面，只有最后一个页面关闭时才会开始记录页面关闭时间
- 页面关闭开始计时、超时自动清理，以及超时前重新打开取消计时时，后端都会输出对应日志，记录用户、开始触发时间、超时时长和超时时间
- 页面关闭时会记录关闭时间；达到 `SESSION_IDLE_TTL_MINUTES` 后，后端会自动清理该账号全部 Token 与对应项目会话缓存；之后再次打开页面会回到登录界面

## 4.3 异步执行机制

- 前端调用 `/api/projects/operate-async` 创建任务
- 后端返回 `job_id`
- 前端轮询 `/api/projects/operate-async/{job_id}` 获取状态
- 支持进度百分比、日志增量、结果文本、结果项列表
- 任务完成后前端根据动作重置表单或保留结果

# 五、环境要求

建议先确认本机运行环境，再执行安装与启动。

| 组件 | 最低要求 | 推荐版本 | 说明 |
| --- | --- | --- | --- |
| Go | 1.24+ | 1.26.x | 后端 `go.mod` 当前声明为 `go 1.26` |
| Node.js | 18+ | 20 LTS | 用于前端开发与构建 |
| npm | 9+ | 10+ | 与 Node.js 版本配套使用 |
| Git | - | 最新稳定版 | 用于代码拉取与发布 |

版本检查命令：

```bash
go version
node -v
npm -v
git --version
```



# 六、本地开发快速启动

## 6.1 克隆项目

```bash
git clone https://github.com/zyx3721/ops-integrated-admin-console.git
cd ops-integrated-admin-console
```

## 6.2 后端配置与启动

1. 进入后端目录下载相关依赖：
```bash
cd backend
go mod tidy
```
2. 配置环境变量：
```bash
# 步骤1：复制模板文件
cp .env.example .env

# 步骤2：编辑 .env，按实际环境修改内网地址、密钥等信息
# 后端服务配置
AD_API_URL=http://ad.example.internal/
PRINT_API_URL=http://print.example.internal/printhub/
VPN_SSH_ADDR=vpn.example.internal
FIREWALL_SSH_ADDR=firewall.example.internal

# 后端运行参数
ADDR=127.0.0.1:8080
PROJECT_CACHE_TTL_MINUTES=10
SESSION_IDLE_TTL_MINUTES=60

# 凭据加密主密钥（建议不少于 16 位，生产环境请替换）
CREDENTIAL_SECRET=change-this-to-your-own-secret-key

# 可选：历史密钥回退列表（用于密钥轮换兼容，多个用英文逗号分隔）
CREDENTIAL_SECRET_FALLBACKS=change-me-ops-credential-secret
```

环境变量说明：

| 参数 | 说明 | 默认值 / 示例 |
| --- | --- | --- |
| `AD_API_URL` | AD 项目后端接口基础地址，用于拼接 AD 相关请求 | 默认 `http://ad.example.internal/` |
| `PRINT_API_URL` | 打印项目后端接口基础地址，用于拼接打印管理请求 | 默认 `http://print.example.internal/printhub/` |
| `VPN_SSH_ADDR` | VPN 系统 SSH 登录地址 | 默认 `vpn.example.internal` |
| `FIREWALL_SSH_ADDR` | 防火墙系统 SSH 登录地址，VPN 联动删除时会使用 | 默认 `firewall.example.internal` |
| `ADDR` | 后端 HTTP 服务监听地址 | 默认 `:8080` |
| `PROJECT_CACHE_TTL_MINUTES` | 项目会话缓存倒计时时长，超过后前端会静默触发项目重登录 | 默认 `10` |
| `SESSION_IDLE_TTL_MINUTES` | 浏览器页面关闭后的空闲超时时长；超过后重新打开页面会要求重新登录。若页面关闭后中途修改该值并重启后端，本次关闭周期通常仍按浏览器里原先保存的旧值判断，下次重新登录后才会按新值生效 | 默认 `60` |
| `CREDENTIAL_SECRET` | 项目凭据加密主密钥，用于加解密数据库中的项目密码 | 无安全默认值，生产环境请务必自定义高强度随机字符串 |
| `CREDENTIAL_SECRET_FALLBACKS` | 历史密钥回退列表，用于密钥轮换后兼容解密旧数据，多个值用英文逗号分隔 | 示例 `old-key-1,old-key-2` |

3. 运行后端服务： 
```bash
# 方式1：前台运行（终端关闭则服务停止）
go run main.go

# 方式2：后台运行（日志输出到 app.log）
nohup go run main.go > app.log 2>&1 &
```

后端服务默认运行在 `http://localhost:8080` ，如需指定端口，请修改环境变量文件内的 `ADDR` 参数。

## 6.3 前端配置与启动

1. 进入前端目录下载相关依赖：
```bash
cd frontend
npm install
```
2. 配置 API 地址（可选）：
```bash
# 配置说明：
# - 后端端口 = 8080：无需创建 .env 文件（默认值为 http://127.0.0.1:8080）
# - 后端端口 ≠ 8080：需要创建 .env 文件（指定正确端口，例如后端端口改为 8090）
#   创建 .env 文件，例如：
echo "VITE_API_BASE=http://localhost:8090" > .env
```
3. 启动前端服务：
```bash
# 方式1：前台运行（终端关闭则服务停止）
npm run dev

# 方式2：后台运行（日志输出到 frontend.log）
nohup npm run dev > frontend.log 2>&1 &
```

前端服务默认运行在 `http://localhost:3000`  ，提供了非本机也能访问，将 `localhost` 改为实际 IP 地址即可。

## 6.4 访问系统

- **首页**：`http://localhost:3000`
- **首次访问**：请先注册管理员账号后再登录

# 七、生产环境部署

## 7.1 克隆项目

```bash
git clone https://github.com/zyx3721/ops-integrated-admin-console.git
cd ops-integrated-admin-console
```

## 7.2 后端构建与配置

1. 进入后端目录下载相关依赖：

```bash
cd backend
go mod tidy
```

2. 配置环境变量：

```bash
# 步骤1：复制模板文件
cp .env.example .env

# 步骤2：编辑 .env，按实际环境修改内网地址、密钥等信息
# 后端服务配置
AD_API_URL=http://ad.example.internal/
PRINT_API_URL=http://print.example.internal/printhub/
VPN_SSH_ADDR=vpn.example.internal
FIREWALL_SSH_ADDR=firewall.example.internal

# 后端运行参数
ADDR=127.0.0.1:8080
PROJECT_CACHE_TTL_MINUTES=15
SESSION_IDLE_TTL_MINUTES=60

# 凭据加密主密钥（建议不少于 16 位，生产环境请替换）
CREDENTIAL_SECRET=change-this-to-your-own-secret-key

# 可选：历史密钥回退列表（用于密钥轮换兼容，多个用英文逗号分隔）
CREDENTIAL_SECRET_FALLBACKS=change-me-ops-credential-secret
```

3. 构建后端可执行文件：

```bash
go build -o ops-backend main.go
```

4. 运行后端服务： 

```bash
# 方式1：前台运行（终端关闭则服务停止）
./ops-backend

# 方式2：后台运行（日志输出到 app.log）
nohup ./ops-backend > app.log 2>&1 &

# 方法3：加入 systemd 管理启动运行
# 服务配置参考如下，请自行修改相应目录路径
cat > /etc/systemd/system/ops-backend.service <<EOF
[Unit]
Description=OPS Integrated Admin Console Service
After=network.target

[Service]
Type=simple
WorkingDirectory=/data/ops-integrated-admin-console/backend
ExecStart=/data/ops-integrated-admin-console/backend/ops-backend

StandardOutput=append:/data/ops-integrated-admin-console/backend/app.log
StandardError=inherit

Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 重载服务配置并启动
systemctl daemon-reload
systemctl start ops-backend

# 设置开机自启
systemctl enable --now ops-backend
```

后端服务默认运行在 `http://localhost:8080` ，如需指定端口，请修改环境变量文件内的 `ADDR` 参数。

## 7.3 前端构建与配置

1. 进入前端目录下载相关依赖：

```bash
cd frontend
npm install
```

2. 构建前端项目：

```bash
npm run build
```

构建产物在 `dist` 目录，可部署到任何静态服务器（Nginx、Vercel、Netlify 等）。生产环境前端无需配置 API 地址，统一通过 Nginx `/api/` 反向代理到后端。

## 7.4 配置Nginx反向代理

在服务器上准备前端目录（例如 `/data/ops-integrated-admin-console/frontend/dist`），**将本地 `dist` 目录中的所有文件和子目录整体上传到该目录**，保持结构不变，例如：

```bash
/data/ops-integrated-admin-console/frontend/dist/
├── assets/
├── favicon.ico
├── index.html
└── vite.svg
```

Nginx 中的 `root` 应指向 **包含 `index.html` 的目录本身**（如 `/data/ops-integrated-admin-console/frontend/dist` ，可按实际路径调整），而不是上级目录。

### 7.4.1 HTTP 示例

> 配置 Nginx （按需替换域名/路径/证书），`HTTP 示例` ：

```nginx
server {
    listen 80;
    server_name your-domain.com;   # 修改为你的域名/主机名，例如：yunwei.cn
    
    # 前端静态资源目录（dist 构建产物）
    root /data/ops-integrated-admin-console/frontend/dist;  # 按实际部署路径修改
    index index.html;
    
    # 限制上传文件大小（可选）
    client_max_body_size 50m;
    
    # 前端路由回退到 index.html（适配前端 history 模式）
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    # 后端 API 反向代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080;  # 与后端 API 相同地址
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
    
    # 健康检查
    location /health {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

### 7.4.2 HTTPS 示例

> HTTPS 示例（含 80→443 跳转，请替换证书路径）：

```nginx
# 80 强制跳转到 443
server {
    listen 80;
    server_name your-domain.com;   # 修改为你的域名/主机名，例如：yunwei.cn
    return 301 https://$host$request_uri;
}

server {
    # listen 443 ssl http2;  # Nginx 1.25 以下版本写法
    listen 443 ssl;
    http2 on;
    server_name your-domain.com;   # 修改为你的域名/主机名，例如：yunwei.cn

    # 证书路径（替换为实际证书文件）
    ssl_certificate     /usr/local/nginx/ssl/your-domain.com.pem;  # 例如：/usr/local/nginx/ssl/yunwei.cn.pem
    ssl_certificate_key /usr/local/nginx/ssl/your-domain.com.key;  # 例如：/usr/local/nginx/ssl/yunwei.cn.key
    
    # SSL安全优化
    ssl_protocols              TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers  on;
    ssl_ciphers                ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
    ssl_session_timeout        10m;
    ssl_session_cache          shared:SSL:10m;

    # 前端静态资源目录（dist 构建产物）
    root /data/ops-integrated-admin-console/frontend/dist;  # 按实际部署路径修改
    index index.html;
    
    # 限制上传文件大小（可选）
    client_max_body_size 50m;
    
    # 前端路由回退到 index.html（适配前端 history 模式）
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    # 后端 API 反向代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080;  # 与后端 API 相同地址
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
    
    # 健康检查
    location /health {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

重载 Nginx：

```bash
# 检查语法
nginx -t

# 重载配置
## 方法1
nginx -s reload
## 方法2
systemctl reload nginx
```

## 7.5 访问系统

- **首页**：`http://your-domain.com`
- **首次访问**：请先注册管理员账号后再登录

- **后端健康检查**：`http://your-domain.com/health` 

# 八、API 文档

本项目后端接口以 `/api` 为主前缀（健康检查同时支持 `/health` 与 `/api/health`）。

## 8.1 接口总览

| 模块 | 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- | --- |
| 健康检查 | GET | `/health` | 否 | 服务健康检查 |
| 健康检查 | GET | `/api/health` | 否 | 兼容前缀形式的健康检查 |
| 认证 | POST | `/api/auth/login` | 否 | 管理员登录 |
| 认证 | POST | `/api/auth/register` | 否 | 管理员注册 |
| 认证 | GET | `/api/auth/me` | 是 | 获取当前登录信息、项目缓存 TTL 与页面关闭超时 TTL |
| 认证 | POST | `/api/auth/window-close-start` | 是 | 记录同浏览器最后一个系统页面关闭后的超时开始信息，并由后端挂起自动清理任务 |
| 认证 | POST | `/api/auth/window-close-cancel` | 是 | 记录超时前重新打开页面后取消关闭计时的日志 |
| 认证 | POST | `/api/auth/logout` | 是 | 退出登录，并清理该账号全部 Token 与对应项目会话缓存 |
| 认证 | POST | `/api/auth/change-password` | 是 | 修改管理员密码 |
| 项目凭据 | GET | `/api/projects/credentials` | 是 | 获取当前管理员项目凭据 |
| 项目凭据 | PUT | `/api/projects/credentials/{project_type}` | 是 | 保存项目凭据（`ad/print/vpn/vpn_firewall`） |
| 项目加载 | POST | `/api/projects/{project}/load` | 是 | 进入项目并建立或复用会话，返回 `session_state` |
| AD 批量模板 | GET | `/api/projects/ad/batch-template` | 是 | 下载 AD 批量创建模板 |
| AD 批量上传 | POST | `/api/projects/ad/batch-upload` | 是 | 上传 AD 批量文件（`multipart/form-data`） |
| AD 批量文件列表 | GET | `/api/projects/ad/batch-files` | 是 | 查询已上传批量文件 |
| 项目操作（同步） | POST | `/api/projects/{project}/operate` | 是 | 基于当前项目会话执行操作，必要时返回 `session_state` |
| 项目操作（异步） | POST | `/api/projects/operate-async` | 是 | 创建异步任务 |
| 异步任务查询 | GET | `/api/projects/operate-async/{job_id}` | 是 | 查询异步执行进度与结果 |
| 项目重登录 | POST | `/api/projects/relogin` | 是 | 清理当前 Token 下项目会话并静默重登，返回 `session_state=countdown_relogin` |
| 操作日志 | GET | `/api/logs` | 是 | 分页查询操作日志 |

## 8.2 鉴权说明

- 除健康检查与登录/注册外，其余接口均需携带：`Authorization: Bearer <token>`。
- 登录成功后返回 `token` 与 `expire_at`，前端据此维护登录态。
- `/api/auth/me` 同时返回 `project_cache_ttl_seconds` 与 `session_idle_ttl_seconds`，前端分别用于缓存倒计时与页面关闭超时判断。
- 项目加载、项目操作、项目重登录接口会返回 `session_state`，当前可见值包括：`first_login`、`reused`、`countdown_relogin`。

## 8.3 项目操作 Action 归类

统一操作接口支持三类项目：`ad`、`print`、`vpn`。操作通过 `action` + `params` 传入。

### 8.3.1 AD 管理（`project_type = ad`）

- `add_user`：新增用户
- `batch_add_users`：批量新增用户
- `search_user`：查询用户
- `reset_password`：重置密码
- `unlock_user`：解锁用户
- `modify_description`：修改描述
- `modify_name`：修改姓名
- `delete_user`：删除用户

### 8.3.2 打印管理（`project_type = print`）

- `add_user`：新增用户
- `search_user`：查询用户
- `get_user`：查询单用户详情（用于修改前回填）
- `reset_password`：重置密码
- `modify_user`：修改用户
- `delete_user`：删除用户

### 8.3.3 VPN 管理（`project_type = vpn`）

- `add_user`：新增用户
- `search_user`：查询用户
- `modify_password`：修改密码
- `modify_status`：修改状态
- `delete_users`：删除用户（支持多用户）
- `export_excel`：当前返回“暂不支持导出功能”

## 8.4 异步接口请求与响应

### 8.4.1 创建异步任务

- 路径：`POST /api/projects/operate-async`
- 示例请求体：

```json
{
  "project_type": "ad",
  "action": "search_user",
  "params": {
    "search_name": "test"
  }
}
```

- 关键响应字段：
  - `job_id`：任务ID
  - `status`：初始状态（`running`）
  - `project_type`、`action`

### 8.4.2 查询异步任务状态

- 路径：`GET /api/projects/operate-async/{job_id}`
- 关键响应字段：
  - `status`：`running/success/failed`
  - `ok`、`done`
  - `progress`、`processed`、`total`
  - `log_lines`：增量日志
  - `result_text`：最终文本结果
  - `result_items`：结构化结果（如批量执行结果）

## 8.5 日志查询参数

`GET /api/logs` 支持：

- `page`：页码（默认 `1`）
- `page_size`：每页条数（默认 `20`，最大 `200`）
- `project_type`：按项目过滤（可选）
- `limit`：兼容参数，等价 `page_size`

# 九、数据库说明

- 固定路径：`backend/db/ops_admin.db`
- 数据库不存在时会自动初始化表结构
- 管理员表为空时不会自动注入默认账号，需由注册接口创建首个管理员
- 主要表：
  - `admins`
  - `auth_tokens`
  - `project_credentials`
  - `operation_logs`
- `project_load_state` 已废弃，旧版本数据库启动时会自动删除该表


# 十、常见问题

## 10.1 `database is locked (SQLITE_BUSY)`

- 原因：多个后端实例同时占用同一个 SQLite 文件
- 处理：结束重复进程，确保仅保留一个后端实例

## 10.2 `decrypt credential failed`

- 原因：`CREDENTIAL_SECRET` 已更换，旧密文无法解密
- 处理：
- 将旧密钥追加到 `CREDENTIAL_SECRET_FALLBACKS`
- 或重新保存一次项目凭据

## 10.3 登录后提示“项目凭据未配置”

- 原因：当前管理员未配置对应项目账号密码
- 处理：先到“项目凭据”页面保存，再进入项目管理

## 10.4 前端中文显示乱码

- 原因：文件编码不是 UTF-8 或终端/编辑器使用了错误编码
- 处理：统一使用 UTF-8 编码保存与查看

## 10.5 登录进入项目后，为什么不是每次操作都重新登录

- 当前实现按“系统 Token + 项目类型”复用项目会话
- 只要项目凭据未变更且未超过 `PROJECT_CACHE_TTL_MINUTES`，后续操作会直接复用现有会话
- 只有首次进入、凭据更新后再次进入，或倒计时到期触发静默重登时，才会重新登录项目

## 10.6 关闭页面超过 1 小时后重新打开会发生什么

- 前端会在同浏览器最后一个系统页面关闭时记录关闭时间
- 达到 `SESSION_IDLE_TTL_MINUTES` 后，后端会自动清理该账号全部 Token 与项目会话缓存，并输出超时清理日志
- 当再次访问时，如果已经超过 `SESSION_IDLE_TTL_MINUTES`，会先回到登录页
- 这里比较的是“当前时间 - 页面关闭时间”与浏览器本地保存的超时阈值，不是后端实时回查
- 如果在超时前重新打开页面，本次页面关闭超时计时会被取消，后端会记录一条“取消计时”的日志
- 如果页面关闭后还没超时，您中途修改了 `SESSION_IDLE_TTL_MINUTES` 并重启后端，那么这一次关闭周期通常仍按旧值判断
- 等这一次因旧值超时而回到登录页后，重新登录时会从后端读取新的 `session_idle_ttl_seconds`，之后下一轮页面关闭超时才会按新值生效
- 前端重新打开页面时仍会做一次本地超时判断与兜底清理，但不再依赖“用户重新访问”才能触发后端清理

## 10.9 同个浏览器开了多个系统页面，关闭其中一个会不会开始算超时

- 当前实现按“同浏览器 + 同 Token”的活动系统页面数量判断
- 只要同浏览器里还有其它系统页面保持打开，关闭其中一个不会真正开始页面关闭超时倒计时
- 只有最后一个系统页面关闭时，才会记录开始触发时间并开始按 `SESSION_IDLE_TTL_MINUTES` 计算
- 最后一个系统页面如果只是立即刷新，后端会将其视为快速恢复，不输出开始计时/取消计时日志
- 当最后一个页面关闭开始计时，以及后续超时自动清理发生时，后端操作日志都会记录对应时间信息，便于排查

## 10.7 同账号打开多个窗口时，缓存倒计时以谁为准

- 同一浏览器下、同一账号且同一 Token 的多个窗口或标签页，共享同一份项目缓存倒计时
- 任一窗口触发倒计时重登后，其它窗口会同步新的倒计时与会话状态
- 不同浏览器、无痕窗口，或重新登录后生成的新 Token，则各自独立计算

## 10.8 清理浏览器 Cookie / 网站数据后，后端 Token 会立即清掉吗

- 前端本地登录态会立即失效，重新访问时会回到登录页
- 但如果浏览器本地数据已被直接清空，前端通常拿不到旧 Token，因此无法再主动调用 `/api/auth/logout`
- 这类情况下，后端 `auth_tokens` 记录不会立刻被前端主动删除；而对应项目会话缓存也不会因为“清浏览器数据”这个动作被即时通知清理

# 十一、安全建议

- 不要将真实生产账号密码提交到 Git
- 发布前建议再次检查 `backend/.env`、`frontend/.env`
- `CREDENTIAL_SECRET` 使用高强度随机字符串并妥善保管
- 定期备份 `backend/db/ops_admin.db`

# 十二、致谢

- 前端基础模板参考：`mars-admin`
- 原业务逻辑参考：`AD_Tool`、`Print_Tool`、`VPN_Tool`（用于迁移对照）

# 十三、外部系统说明

- VPN 管理调用的是天融信下一代 VPN 平台系统。
- 打印管理调用的是迅维打印机管理系统。
- AD 管理调用的是开源项目 ITOPS（经过二开）：https://github.com/openitsystem/itops 。

# 十四、移动端适配说明

为提升手机和平板访问体验，前端已新增响应式适配，且不改变后端接口与业务逻辑。

## 14.1 适配范围

- 登录/注册页面：窄屏下自动切换为更适合触屏输入的表单布局。
- 后台主界面：移动端使用抽屉菜单替代左侧常驻导航栏。
- 项目管理界面：移动端增加“功能选择”下拉切换，替代功能侧栏。
- 项目凭据页面：卡片网格根据屏幕宽度自动切换列数。
- 操作日志页面：表格支持横向滚动，避免窄屏下内容挤压错位。

## 14.2 响应式行为

- <= 900px：进入移动端主布局（抽屉导航 + 顶部菜单按钮）。
- <= 760px：项目凭据卡片按 1 列展示。
- 761px ~ 1260px：项目凭据卡片按 2 列展示。
- > 1260px：项目凭据卡片按 3 列展示。

## 14.3 使用建议

- 移动端优先使用系统浏览器最新版本（Chrome/Safari/Edge）。
- 若页面样式未刷新，建议清理浏览器缓存后重新访问。
- 生产部署建议使用 HTTPS，确保移动端会话与凭据传输安全。

# 十五、许可证

本项目采用 [MIT License](LICENSE) 开源协议。

MIT License 是一个宽松的开源许可证，允许您自由地使用、复制、修改、合并、发布、分发、再许可和/或销售本软件的副本。唯一的要求是在所有副本或重要部分中保留版权声明和许可声明。
