# EasyTier + Cloudflare 中心节点

用官方 [EasyTier](https://easytier.cn/) 在任意一台**没有公网 IP** 的机器上跑共享中心节点，再经 [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) 对外提供 `wss://` 入口。远程设备连上中心节点后自动组虚拟网，需要访问家里/公司局域网时再加一个子网出口节点。

```text
远程客户端  --wss://et.example.com-->  Cloudflare 边缘
                                          |
                                     Cloudflare Tunnel
                                          |
中心机器  cloudflared  →  easytier-core (仅本机 127.0.0.1:11011)
                                          |
家宽出口节点 (可选)  -- 导出 192.168.1.0/24 -->  远程访问内网
```

中心节点开 `--no-tun`，不创建虚拟网卡，不需要 root，也不对外开放任何入站端口。

## 前置条件

- 一台能 24 小时在线、能访问外网 443/7844 的 Linux 机器（NAS、旁路网关、VPS 均可）
- Docker 与 Docker Compose，或直接安装 `easytier-core` + `cloudflared`
- Cloudflare 账号，域名 NS 已托管到 Cloudflare（仅 CNAME 接入不够）
- 免费计划即可；控制台 **Network → WebSockets** 保持开启

## 1. 创建 Cloudflare Tunnel

1. 打开 [Zero Trust → Networks → Tunnels](https://one.dash.cloudflare.com/)，点 **Create a tunnel**，选 Cloudflared，名称例如 `easytier-center`。
2. 复制安装命令里的 **Token**，稍后写入 `.env` 的 `TUNNEL_TOKEN`。
3. 添加一条 **Published application** 路由：
   - Subdomain / Domain：例如 `et.example.com`
   - Type：`HTTP`
   - URL：`http://127.0.0.1:11011`
   - Cloudflare 会自动处理 WebSocket 升级，不要填 `ws://` / `wss://`
4. SSL/TLS 模式用 **Full** 即可（Tunnel 出站由 Cloudflare 终结 TLS）。

## 2. 启动中心节点

```bash
git clone https://github.com/lm-lovett/intranet-penetration-cloudflare.git
cd intranet-penetration-cloudflare
cp .env.example .env
```

编辑 `.env`：填入网络名、密钥、公网域名和 Tunnel Token。然后：

```bash
docker compose up -d
docker compose logs -f
```

本机确认中心节点起来：

```bash
docker exec easytier-center easytier-cli node
docker exec easytier-center easytier-cli peer
```

没有 Docker 时，把 `conf/easytier.toml` 和 `.env` 拷到 `/etc/easytier/`（`.env` 命名为 `center.env`），安装官方 `easytier-core` 与 `cloudflared`，启用 `systemd/` 下两个 unit。

## 3. 客户端接入

网络名和密钥必须与中心节点一致。不要在 peer 地址里写 `:0` 端口。

```bash
easytier-core \
  -d \
  --network-name office \
  --network-secret '你的密钥' \
  -p 'wss://et.example.com'
```

或使用仓库里的配置文件：

```bash
cp examples/client.toml /tmp/client.toml
# 改 network_secret 和 peer uri
easytier-core -c /tmp/client.toml
```

手机 App：网络方式选手动，服务器填 `wss://et.example.com`。

## 4. 内网穿透（导出局域网）

中心节点只做发现和中继。要让远程机器访问家里的 `192.168.1.0/24`，在一台已经能到达该网段的设备上启动出口节点：

```bash
sudo easytier-core -c examples/lan-exit.toml
```

把 `[[proxy_network]]` 的 CIDR 改成实际网段。远程客户端加入同一虚拟网后，即可访问该网段里的主机。

## Docker Desktop（macOS / Windows）

host 网络不可用时改用 bridge 编排，并在 Cloudflare 把 Service URL 改成 `http://easytier:11011`：

```bash
docker compose -f docker-compose.bridge.yml up -d
```

## 临时域名试跑

没有自己的域名时可以用 `*.trycloudflare.com`：

```bash
docker compose -f docker-compose.yml -f docker-compose.quick.yml up
```

从 cloudflared 日志里复制 `https://xxxx.trycloudflare.com`，把 `.env` 里的 `EASYTIER_PUBLIC_HOST` 改成同样的主机名（不要带协议），客户端用 `wss://xxxx.trycloudflare.com`。

## 配置说明

| 变量 | 作用 |
| --- | --- |
| `EASYTIER_NETWORK_NAME` / `EASYTIER_NETWORK_SECRET` | 虚拟网身份，所有节点必须相同 |
| `EASYTIER_PUBLIC_HOST` | Cloudflare 对外域名，写入 EasyTier `mapped_listeners` |
| `EASYTIER_HOSTNAME` | 中心节点在 peer 列表里的名字 |
| `TUNNEL_TOKEN` | Zero Trust 下发的 Tunnel 令牌 |

中心节点默认：

- 只监听 `ws://127.0.0.1:11011`，不暴露公网端口
- `private_mode`：拒绝其它网络名/密钥的节点
- `no_tun`：纯中继，不占用虚拟 IP
- `mapped_listeners = ["wss://你的域名/"]`：把 Cloudflare 入口通告给对端

## 故障排查

| 现象 | 处理 |
| --- | --- |
| Cloudflare 502 | `cloudflared` 未连上，或 EasyTier 没在 `127.0.0.1:11011` 监听 |
| 客户端连不上 WSS | 确认 WebSockets 开启；peer 写成 `wss://域名` 不要带 `:0` |
| GUI 连 WSS 失败、CLI 成功 | Cloudflare 需支持 TLS 1.3（默认已开） |
| 能进虚拟网但访问不了局域网 | 出口节点要加 `proxy_network`，且和客户端同一 `network_name` |
| Docker 里 fd 耗尽 | compose 已设 `LimitNOFILE`；systemd 同样需要 |

## 不自建机器时

不想跑 24 小时主机，可以把中继直接放到 Cloudflare Workers 边缘：[easytier-edge](https://github.com/fordes123/easytier-edge)。客户端同样用 `wss://你的 Worker 域名/`，但必须开 `--secure-mode` 并配置网络白名单密钥。本仓库默认方案仍是官方 `easytier-core` + Tunnel，协议兼容性更好。
