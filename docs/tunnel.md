# 可选：本机 EasyTier + Cloudflare Tunnel

Workers 方案不需要这套。只有当你想用官方 `easytier-core`（不必开 `secure-mode`）时，才在一台 24 小时在线的机器上跑中心节点，再用 Tunnel 把 `ws://127.0.0.1:11011` 暴露成 `wss://`。

## 不用 Docker

安装 [EasyTier](https://easytier.rs/guide/installation.html) 和 [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)，拷配置：

```bash
sudo mkdir -p /etc/easytier
sudo cp conf/easytier.toml /etc/easytier/easytier.toml
sudo cp .env.example /etc/easytier/center.env   # 填网络名、密钥、域名、TUNNEL_TOKEN
sudo cp systemd/easytier-center.service systemd/cloudflared.service /etc/systemd/system/
sudo systemctl enable --now easytier-center cloudflared
```

Cloudflare Zero Trust 里创建 Tunnel，Published application 指到 `HTTP http://127.0.0.1:11011`。

## 用 Docker

Linux 主机网络：

```bash
cp .env.example .env
docker compose up -d
```

Docker Desktop 用 `docker-compose.bridge.yml`，并把 Tunnel 的 Service URL 改成 `http://easytier:11011`。
