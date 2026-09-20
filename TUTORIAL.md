# cfpen 使用教程

这是一份从零开始的操作说明：在家里或公司内网装一次 `cfpen`，外网就能访问内网网站，或把整段内网流量走回家。

把文中的 `example.com` 换成你自己已经接到 Cloudflare 的域名。

## 1. 它能做什么

`cfpen` 是同一个程序，内网和外网各装一份，底层走官方 Cloudflare Tunnel（`cloudflared`），不需要在路由器上开端口。

两条互不冲突的路：

| 场景 | 外网怎么用 | 适合 |
| --- | --- | --- |
| 公网 HTTPS | 浏览器打开 `https://app.example.com` | 内网网站、开发环境、NAS Web |
| 双端代理 | 外网本机 `127.0.0.1:1080` 当 SOCKS 代理 | 访问 `192.168.x.x`、SSH、打印机管理页、经家里出口上网 |

```text
外网浏览器 ──HTTPS──► Cloudflare ──隧道──► 内网 cfpen ──► 127.0.0.1:8080
外网程序   ──SOCKS──► 本机 cfpen connect ──Access──► 内网 SOCKS ──► 192.168.1.1
```

## 2. 开始前准备

1. 一个 Cloudflare 账号。  
2. 一个域名已经添加到 Cloudflare，DNS 由 Cloudflare 代理（橙色云）。没有域名就无法发布 `https://xxx.你的域名`。  
3. 内网机器能访问互联网（只需出站，不要映射端口）。  
4. 第一次用 Zero Trust / Access 时，先打开一次 [Cloudflare One](https://one.dash.cloudflare.com)，按提示创建一个 Team，否则后面保护内网代理可能失败。

本机需要能编译 Go 程序，或使用别人编好的 `cfpen` 二进制。

## 3. 安装

在能上网的机器上：

```bash
git clone https://github.com/lm-lovett/intranet-penetration-cloudflare.git
cd intranet-penetration-cloudflare
go build -o cfpen ./cmd/cfpen
```

把 `cfpen` 拷到：

- 内网那台要当入口的机器（能访问 NAS、路由器、开发服务的那台）
- 外网你自己用的电脑

Linux / macOS 建议放到 PATH 里：

```bash
sudo install -m 755 cfpen /usr/local/bin/cfpen
cfpen version
```

第一次 `up` / `connect` 时会自动把官方 `cloudflared` 下载到 `~/.cfpen/bin/`。

## 4. 第一次登录（内网机器）

在**内网机器**上执行：

```bash
cfpen login --zone example.com
```

会发生什么：

1. 本机弹出一个 cfpen 登录页，同时打开 Cloudflare 网站。  
2. 如果还没登录 Cloudflare，先完成账号登录（邮箱、Google、Passkey 都可以）。  
3. 登录后进入「创建 API Token」页，权限已经按 cfpen 预填好。  
4. 点 **Create Token**，复制 Token。  
5. 回到本机 cfpen 页面（或终端）粘贴 Token，点完成。

没有桌面、只能 SSH 时：

```bash
cfpen login --no-browser --zone example.com
```

终端会打印 Cloudflare 链接。用手机或另一台有浏览器的电脑打开链接，创建 Token 后，把 Token 粘回终端回车。

已经有 Token 时可以跳过浏览器：

```bash
cfpen login --token "$CLOUDFLARE_API_TOKEN" --zone example.com
```

成功后会看到账号名和 zone。凭证写在 `~/.cfpen/credentials.json`（权限 0600），不要把这个文件拷到外网。

账号下有多个域名时，必须加 `--zone`，否则会用列表里的第一个。

## 5. 教程 A：把内网网站发到公网

适合：家里 NAS、本地 `localhost:3000` 的前端、公司内网后台（确认可以公开或另加鉴权）。

### 5.1 确认源站在听

例如本机 8080 已经有服务：

```bash
curl -I http://127.0.0.1:8080
```

### 5.2 打洞

```bash
cfpen up
```

保持这个终端不要关。成功时大致会看到：

- `dashboard: http://127.0.0.1:4090`
- `intranet socks listening on 127.0.0.1:41080`
- `client bundle: ~/.cfpen/client.json`

另开一个终端发布端口：

```bash
cfpen expose 8080 --hostname app.example.com
```

也可以：

```bash
# 只写端口，会发布成 https://web.example.com
cfpen expose 8080 --name web

# 指定内网其它机器
cfpen expose 192.168.1.10:80 --hostname nas.example.com

# HTTPS 源站
cfpen expose 127.0.0.1:443 --protocol https --hostname secure.example.com
```

浏览器打开 <http://127.0.0.1:4090>，也能在网页里填写端口并暴露。

### 5.3 验证

等几十秒 DNS 生效后：

```bash
curl -I https://app.example.com
```

外网手机流量、公司网络，直接用浏览器打开该地址即可，**外网不用装 cfpen**。

### 5.4 注意

`https://app.example.com` 默认是公开的，知道网址就能访问。不要把没有登录墙的管理后台直接暴露出去。需要收口时，到 Cloudflare Zero Trust 给这个 hostname 再加 Access 策略。

停掉发布：

```bash
cfpen unexpose app
# 或
cfpen unexpose app.example.com
```

## 6. 教程 B：外网访问整个内网

适合：外网 SSH 到 `192.168.1.2`、打开路由器 `192.168.1.1`、访问没有公网域名的内网服务。

### 6.1 内网保持 `cfpen up`

教程 A 里如果已经 `up` 过，不要关。`up` 会同时：

- 在内网 `127.0.0.1:41080` 开 SOCKS
- 把 `proxy.example.com` 指过去
- 用 Cloudflare Access Service Auth 锁住这个入口
- 写出 `~/.cfpen/client.json`

只想做公网网站、不要代理时：

```bash
cfpen up --no-proxy
```

### 6.2 把 client.json 拷到外网

只要拷 **client.json**，不要拷 `credentials.json`。

```bash
# 内网机器上也可以显式导出一份
cfpen export-client ./client.json
```

用 U 盘、scp、密码管理器均可。`client.json` 等于一把能转发任意 TCP 的钥匙，按密钥保管。

### 6.3 外网连接

```bash
cfpen connect --token-file client.json
```

默认本机代理是 `127.0.0.1:1080`。换端口：

```bash
cfpen connect --token-file client.json --listen 127.0.0.1:1088
```

### 6.4 验证

另开终端：

```bash
# 访问内网网关（按你家实际 IP 改）
curl --proxy socks5://127.0.0.1:1080 http://192.168.1.1

# 让当前命令都走内网
ALL_PROXY=socks5://127.0.0.1:1080 curl http://10.0.0.2:9000
```

浏览器：设置 HTTP / SOCKS 代理为 `127.0.0.1` 端口 `1080`，即可打开内网网址。

SSH 示例：

```bash
ssh -o ProxyCommand="ncat --proxy 127.0.0.1:1080 --proxy-type socks5 %h %p" user@192.168.1.2
```

没有 `ncat` 时，也可以用 `cfpen expose` 把 SSH 单独发到一个 hostname（见下一节）。

## 7. SSH / RDP / 其它 TCP

不想开系统代理、只穿透一个端口时，在内网：

```bash
cfpen expose 22 --protocol ssh --hostname ssh.example.com
```

外网再自己用 `cloudflared access tcp` 或把该 hostname 配到 SSH `ProxyCommand`。更省事的方式仍是教程 B：`connect` 之后直接 `ssh user@192.168.x.x`（需 SSH 客户端走 SOCKS）。

RDP、SMB 同理：`--protocol rdp` / `--protocol tcp`。

## 8. 日常命令

这些命令在内网、`up` 可以同时在另一个终端执行（配置改的是 Cloudflare 侧，正在跑的隧道会跟上）：

```bash
cfpen ls          # 已发布的公网服务
cfpen status      # 是否在跑、tunnel 连接数、代理地址
cfpen down        # 停止 up 或 connect
```

关掉 `up` 所在终端（Ctrl+C）等价于停隧道；也可以在另一个窗口 `cfpen down`。

## 9. 文件分别是什么

| 路径 | 在哪台机器 | 能不能外传 |
| --- | --- | --- |
| `~/.cfpen/credentials.json` | 仅内网 | 否，含 API Token |
| `~/.cfpen/config.yaml` | 内网 | 可以，无密钥 |
| `~/.cfpen/client.json` | 内网生成，拷到外网 | 可以，但要保密 |
| `~/.cfpen/bin/cloudflared` | 两边自动下载 | 官方程序 |

配置文件还会找当前目录的 `cfpen.yaml`。常用环境变量：`CLOUDFLARE_API_TOKEN`、`CLOUDFLARE_ACCOUNT_ID`、`CLOUDFLARE_ZONE_ID`。

## 10. 排错

**浏览器没弹出来**  
看终端里的 `local:` 和 `cloudflare:` 两行 URL，手动打开。或改用 `cfpen login --no-browser`。

**login 报 token 权限不足**  
Token 需要：Account · Cloudflare Tunnel · Edit，Zone · DNS · Edit，Account · Access: Apps and Policies · Edit，Account Settings · Read。用 `cfpen login` 预填模板最省事。

**up 提示 proxy disabled / Access 失败**  
打开 [one.dash.cloudflare.com](https://one.dash.cloudflare.com) 创建 Team 后再 `cfpen up`。公网 HTTPS 仍可用；只是双端代理没挂上。

**https://app.example.com 打不开**  

- `cfpen status` 看隧道是否 connected  
- 域名是否就是 `--zone` 那个 zone 下的  
- DNS 是否已生成 CNAME 到 `*.cfargotunnel.com` 且已代理  
- 源站 `curl 127.0.0.1:端口` 在内网是否正常  

**connect 后代理连不上内网 IP**  

- 内网 `cfpen up` 必须还在跑  
- 用的是最新 `client.json`  
- 目标 IP 对内网那台机器要路由得通（`cfpen` 装在能访问该 IP 的主机上）  

**想换域名或重来**  
`cfpen down` 后重新 `cfpen login --zone 新域名` 再 `up`。旧 hostname 的 DNS 记录可能还在 Cloudflare 里，可在 Dashboard 里删掉。

## 11. 安全建议

- 只在自己的内网机器上跑 `up`。  
- HTTP 发布默认全世界可访问，敏感服务先加应用自己的登录，或再套 Cloudflare Access。  
- `client.json` 丢失就当作密钥泄露：在 Cloudflare Zero Trust 里吊销对应 Service Token，再 `cfpen up` 一次生成新的。  
- 不要把 API Token 发到聊天软件或提交进 git。
