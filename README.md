# cfpen

基于 Cloudflare Tunnel 的双模式内网穿透工具：同一个 `cfpen` 二进制，内网负责打洞，外网负责把请求送回内网。

## 两种访问方式

1. **公网 HTTPS（ngrok 风格）**  
   内网把某个本地端口发布成 `https://app.example.com`，外网用浏览器直接打开，不必装客户端。

2. **双端代理（访问整个内网）**  
   内网再开一个仅环回的 SOCKS5；Cloudflare Access Service Token 保护 `proxy.example.com`。外网运行 `cfpen connect`，本机 `127.0.0.1:1080` 成为指向内网的 SOCKS5/HTTP 代理。浏览器或 `ALL_PROXY` 走这个代理后，可以访问 `192.168.x.x`、内网网站，以及经内网出口访问公网。

数据面使用官方 `cloudflared`（自动下载到 `~/.cfpen/bin/`），不自建 Worker 中继。

## 前提

- Cloudflare 账号，并且有一个已经接入 Cloudflare 的域名（Zone）
- API Token，建议权限：
  - Account · Cloudflare Tunnel · Edit
  - Zone · DNS · Edit
  - Account · Access: Apps and Policies · Edit
  - Account · Account Settings · Read（用于列出账号）
- 内网机器能访问互联网（只需出站，不需要在路由器上开端口）
- 首次使用 Zero Trust / Access 时，如 API 报错，先打开一次 [Cloudflare One](https://one.dash.cloudflare.com) 创建 Team

## 安装

```bash
go build -o cfpen ./cmd/cfpen
```

把 `cfpen` 拷到内网机器和外网机器即可。

## 内网：登录并启动

```bash
cfpen login --zone example.com
cfpen up
```

`cfpen login` 会打开浏览器进入 Cloudflare 登录页（未登录会先登录），并预填创建 Token 所需权限。在 Dashboard 里点 Create Token，把 Token 粘贴回本机打开的 cfpen 页面或终端即可。

无图形界面时用 `--no-browser`，终端会打印 Cloudflare 链接，把 Token 粘贴回终端：

```bash
cfpen login --no-browser --zone example.com
```

也可以继续手动传入 Token：

```bash
cfpen login --token "$CLOUDFLARE_API_TOKEN" --zone example.com
```

默认会：

- 创建或复用名为 `cfpen` 的远程管理 Tunnel
- 启动本地控制台：<http://127.0.0.1:4090>
- 在 `127.0.0.1:41080` 启动 SOCKS5/HTTP CONNECT
- 把 `proxy.example.com` 指到该代理，并用 Access **Service Auth** 保护
- 在 `~/.cfpen/client.json` 写出外网凭证（不含 API Token）

发布一个网站：

```bash
cfpen expose 8080 --hostname app.example.com
# 或
cfpen expose 127.0.0.1:3000 --name lab
# -> https://lab.example.com
```

查看 / 停止：

```bash
cfpen ls
cfpen status
cfpen down
```

HTTP 服务默认是**公开 HTTPS**（任何人知道域名就能访问）。不要暴露未加鉴权的管理后台。如需收口，可在 Cloudflare Zero Trust 里给该 hostname 另加 Access 策略。

## 外网：连接内网代理

把内网生成的 `client.json` 拷到外网机器（不要拷 `credentials.json`，那里有 API Token）。

```bash
cfpen connect --token-file client.json
```

本地代理默认 `127.0.0.1:1080`。验证：

```bash
curl --proxy socks5://127.0.0.1:1080 http://192.168.1.1
ALL_PROXY=socks5://127.0.0.1:1080 curl http://10.0.0.2:9000
```

浏览器把 HTTP/SOCKS 代理设为 `127.0.0.1:1080` 后，访问内网资源即可。

导出凭证：

```bash
cfpen export-client ./client.json
```

## 配置

配置文件搜索顺序：`./cfpen.yaml`、`./cfpen.yml`、`~/.cfpen/config.yaml`。

```yaml
account_id: ""
zone_id: ""
zone_name: example.com
tunnel_name: cfpen
dashboard:
  listen: 127.0.0.1:4090
proxy:
  hostname: proxy.example.com
  listen: 127.0.0.1:41080      # 内网 SOCKS
  client_listen: 127.0.0.1:1080 # 外网本地端口
services:
  - name: app
    hostname: app.example.com
    service: http://127.0.0.1:8080
```

敏感信息在 `~/.cfpen/credentials.json`（权限 0600），外网只用 `client.json`。

常用环境变量：`CLOUDFLARE_API_TOKEN`、`CLOUDFLARE_ACCOUNT_ID`、`CLOUDFLARE_ZONE_ID`。

## 安全说明

- 内网 SOCKS 只绑在 `127.0.0.1`，公网只能打到 `proxy.<zone>`，且必须带 Access Service Token
- `client.json` 等同于一把能转发任意 TCP 的钥匙，按密钥保管
- 本工具只把**你主动发起**的请求送到内网，用于访问自己的网络资源

## 命令一览

| 命令 | 作用 |
| --- | --- |
| `cfpen login` | 打开浏览器登录 Cloudflare，或使用 `--token` |
| `cfpen up` | 内网启动 Tunnel + SOCKS + 控制台 |
| `cfpen expose` | 发布 HTTP/HTTPS/TCP/SSH/RDP 到公网 hostname |
| `cfpen unexpose` | 取消发布并删除对应 DNS |
| `cfpen connect` | 外网接入，打开本地代理 |
| `cfpen export-client` | 生成可拷贝的 `client.json` |
| `cfpen ls` / `status` / `down` | 列表、状态、停止 |

`--no-proxy` 可在 `up` 时只做公网 HTTPS、不发布内网代理。
