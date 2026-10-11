# AI Arsenal LLM Router

OpenAI 兼容的 LLM 网关（gin），独立 go module。鉴权/路由/翻译/计费完整实现。

## 域名与环境

| 环境 | 域名 | 启动 |
|------|------|------|
| 测试 | `llm.aiarsenal.cc` | `APP_ENV=test go run ./cmd`（HTTP + /etc/hosts） |
| 生产 | `llm.aiarsenal.cc` | `APP_ENV=prod go run ./cmd` |
| 自定义 | 任意 | `ROUTER_DOMAIN=xxx.com APP_ENV=prod go run ./cmd` |

`APP_ENV` 决定默认域名；`ROUTER_DOMAIN` 可覆盖任意环境域名。`/health` 暴露当前 env/domain：
```
$ curl http://llm.aiarsenal.cc/health
{"status":"ok","env":"test","domain":"llm.aiarsenal.cc","origin":"https://llm.aiarsenal.cc"}
```

服务监听 `:43214`（`PORT` 可改），域名由前置 nginx/反代终止 TLS 并转发到 `127.0.0.1:43214`。

## 配置（全 env 覆盖，默认值对齐本机测试环境）

| 变量 | 默认 | 说明 |
|------|------|------|
| `APP_ENV` | `test` | `test`/`prod` |
| `ROUTER_DOMAIN` | 按 env | 公开域名（CORS 与 URL 生成） |
| `ROUTER_SCHEME` | `https` | 对外协议 `http`/`https`（CORS Origin 前缀；测试可设 `http`） |
| `PORT` | `43214` | 监听端口 |
| `DB_HOST/PORT/USER/PASSWORD/NAME` | 127.0.0.1/5432/aiarsenal/aiarsenal123/llm | llm 库 |
| `REDIS_ADDR/DB/PASSWORD` | 127.0.0.1:6379/2/ | redis（db=2，区别于 api 的 db=1） |
| `SECRET_KEY` | （随机生成） | 与 api 的 `jwt.encryptKey` 一致，解密 provider 凭据；部署时由 `/etc/aiarsenal/env` 注入，不入库 |

## nginx 反代（测试环境 llm.aiarsenal.cc，纯 HTTP）

测试环境不走 HTTPS：用 /etc/hosts 把 `llm.aiarsenal.cc` 指向 `127.0.0.1`，nginx 纯 HTTP 转发。

```nginx
server {
    listen 80;
    server_name llm.aiarsenal.cc;

    # 流式响应必须关缓冲
    proxy_buffering off;
    proxy_read_timeout 600s;

    location / {
        proxy_pass http://127.0.0.1:43214;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

生产环境走 HTTPS（443 ssl + 真实证书），完整配置见 `deploy/nginx/prod.conf`。

## systemd 常驻（示例）

```ini
# /etc/systemd/system/aiarsenal-llm.service
[Unit]
Description=AI Arsenal LLM Router
After=network.target postgresql.service redis.service

[Service]
WorkingDirectory=/home/ubuntu/aiarsenal/llm
Environment="APP_ENV=test"
ExecStart=/home/ubuntu/aiarsenal/llm/bin/llm
Restart=on-failure
User=ubuntu

[Install]
WantedBy=multi-user.target
```

## 调用

```bash
curl -H "Authorization: Bearer <明文virtual key>" \
     -H "Content-Type: application/json" \
     -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"stream":true}' \
     http://llm.aiarsenal.cc/v1/chat/completions
```

## 目录结构

```
llm/
├── cmd/main.go              # gin 装配 + 启动
└── internal/
    ├── config/              # env 配置 + env/domain 推导
    ├── db/                  # gorm(postgres) + redis
    ├── model/               # llm 库只读实体（virtual_key/model/credential）
    ├── store/               # DB 查询 + 凭据解密 + 计费写入
    ├── crypto/              # AES-256-GCM（与 api 一致）
    ├── auth/                # Bearer → sha256 → 查库 → budget/expires 校验
    ├── middleware/          # CORS
    ├── routing/             # round-robin + cooldown
    ├── provider/            # 翻译层接口 + OpenAI 兼容实现
    └── proxy/               # /v1/chat/completions /v1/embeddings /v1/models
```
