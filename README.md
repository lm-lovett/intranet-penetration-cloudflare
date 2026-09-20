# EasyTier 中心节点（Cloudflare Workers）

不需要 Docker，也不需要自己的 24 小时主机。中心节点直接跑在 Cloudflare 边缘：`wrangler deploy` 之后，客户端用 `wss://你的 Worker 域名:443/` 接入。

```text
远程客户端  --wss://easytier-center.<账号>.workers.dev-->  Cloudflare Worker
                                                              │
                                                     Durable Object 中继
                                                              │
家宽出口节点 (可选)  -- 导出 192.168.1.0/24 -->  远程访问内网
```

Worker 实现来自 [easytier-edge](https://github.com/fordes123/easytier-edge)（LGPL-3.0），协议走 Noise XX + `secure-mode`。本仓库 Worker 名是 `easytier-center`。

## 前置条件

- Node.js 20+ 和 [pnpm](https://pnpm.io/)
- Cloudflare 账号（免费计划即可）
- 本机第一次部署需要 `wrangler login`；CI 用 API Token

Rust 只在**构建 WASM** 时需要。`cloudflare/scripts/build-wasm.mjs` 会按 `rust-toolchain.toml` 使用 1.95.0 + `wasm32-unknown-unknown`。没有的话先装 [rustup](https://rustup.rs/)。

## 1. 部署到 Cloudflare

```bash
cd intranet-penetration-cloudflare
npm install -g pnpm
./scripts/cf-deploy.sh login
```

生成网络密码（自己记下来）和服务端密钥，再写入 Worker Secret：

```bash
export EASYTIER_NETWORK_NAME=office
export EASYTIER_NETWORK_SECRET='换成足够长的随机串'
./scripts/cf-deploy.sh secrets
./scripts/cf-deploy.sh
```

`secrets` 会把公钥打在终端上。客户端如需校验中心节点身份，把该公钥配到 `peer_public_key`。

部署成功后 Wrangler 会打印类似：

```text
https://easytier-center.<你的子域>.workers.dev
```

健康检查：打开 `https://easytier-center.<子域>.workers.dev/healthz`，应返回 `"ok": true`。

### 认领临时预览账号

`wrangler deploy --temporary` 会先建一个独立的预览账号（例如 `Zesty Nautilus`），再给你认领链接。认领后这个预览账号会变成你名下的**第二个 Cloudflare 账号**，Worker 不会自动出现在原来那个已经托管域名的账号里。

- 在 Dashboard 左上角账号切换器里找预览账号名称。
- 要把中心节点放到已有账号：对本机执行 `./scripts/cf-deploy.sh login`，选中已有账号，再跑 `secrets` 和 `deploy`（不要加 `--temporary`）。绑定自定义域名也必须在那个已有账号上做。

### 绑定自己的域名（可选）

Cloudflare Dashboard → Workers → `easytier-center` → Settings → Domains & Routes，加上例如 `et.example.com`。客户端 peer 改成 `wss://et.example.com/`。

## 2. 客户端接入

必须开 `--secure-mode`，网络名/密钥与 Secret 里的 `EASYTIER_NETWORKS` 一致。Worker 只开 **443**。EasyTier GUI 里 WSS 默认端口是 **11012**，不改就会 `connect timeout`。

### easytier-gui

打开 **添加新网络 / 配置网络**，不要选「公共服务器」（那是官方共享节点，连不上这个 Worker）。

| GUI 字段 | 填什么 |
| --- | --- |
| 网络方式 | **手动** |
| 初始节点 | 协议 **wss**，主机 `easytier-center.<子域>.workers.dev`，端口 **443** |
| 或一整条 URI | `wss://easytier-center.<子域>.workers.dev:443/` |
| 网络名称 | 部署时的 `EASYTIER_NETWORK_NAME`（例如 `office`） |
| 网络密码 | 部署时的 `EASYTIER_NETWORK_SECRET` |
| 虚拟IPv4 / DHCP | 打开 **DHCP** |
| 高级设置 → 禁用加密 | **不要勾** |
| 高级设置 → 启用私有模式 | 可关；这不是安全模式 |

保存后再打开配置，确认端口仍是 **443**，不是 `11012`。有的 GUI 会把 443 改回 11012，这时用 **编辑配置文件** 写成：

```toml
[[peer]]
uri = "wss://easytier-center.<子域>.workers.dev:443/"
```

输入初始节点后要点一下列表项确认，确认后地址会变成卡片。然后点 **运行网络**。

连这个 Worker **不需要** WireGuard 入站。GUI 默认会监听 `wg://0.0.0.0:11011`，Windows 上若端口已被另一个 EasyTier 实例占用，会报 `监听器添加失败` / `AddrInUse (10048)`。到 **高级设置 → 监听地址** 删掉 `wg://0.0.0.0:11011` 即可；这条失败一般会 `retry listen later`，TCP/UDP 和去 Worker 的 `wss://` 仍能用。真要留 WG，把端口改成空闲的（例如 `wg://0.0.0.0:11021`），或多开网络时每个实例用不同端口。

**安全模式必须开**，否则握手失败（`same-network peers must use the same secure mode`）：

- 新版 GUI：展开 **高级设置 → 功能开关**，打开 **安全模式 / Secure Mode**（在「禁用加密」旁边）。不要把它和「禁用加密」「私有模式」搞混。
- 旧版 GUI 没有这个开关：点 **显示配置 / 编辑配置文件**，在文件里加上 `secure_mode = true`（或 `[secure_mode]` 段里 `enabled = true`），保存后再运行。如果一点保存这一行就消失，换较新的 GUI，或直接用下面的 `easytier-core`。

可选：把部署时打印的中心节点公钥配到 `peer_public_key`，用来锁定 Worker 身份。GUI 里若没有这项，同样写进配置文件。

家宽出口节点再填 **子网代理CIDR**，例如 `192.168.1.0/24`。

### 命令行

```bash
easytier-core \
  -d \
  --network-name office \
  --network-secret '和部署时相同的密钥' \
  --secure-mode \
  -p 'wss://easytier-center.<子域>.workers.dev:443/'
```

或改仓库里的示例：

```bash
cp examples/client.toml /tmp/client.toml
# 改 network_secret 和 [[peer]] uri
easytier-core -c /tmp/client.toml
```

每台客户端可以再生成一对密钥并写进配置，重启后身份才稳定：

```bash
cd cloudflare && pnpm run keys
```

把输出的 `LOCAL_PRIVATE_KEY` / `LOCAL_PUBLIC_KEY` 填进客户端的 `--local-private-key` / `--local-public-key`。

## 3. 内网穿透（导出局域网）

Worker 只做发现和中继，没有 TUN。要访问家里 `192.168.1.0/24`，在一台已经能到达该网段的设备上启动出口节点：

```bash
sudo easytier-core -c examples/lan-exit.toml
```

把 `[[peer]]` 改成你的 Worker `wss://` 地址，把 `[[proxy_network]]` 改成实际网段。

## 配置项

写入 Cloudflare 的三个 Secret：

| Secret | 含义 |
| --- | --- |
| `EASYTIER_NETWORKS` | JSON 数组，例如 `[{"network_name":"office","network_secret":"..."}]` |
| `LOCAL_PRIVATE_KEY` | 中心节点 X25519 私钥（Base64） |
| `LOCAL_PUBLIC_KEY` | 对应公钥；客户端可 pin |

`wrangler.jsonc` 里的普通变量：

| 变量 | 默认 | 含义 |
| --- | --- | --- |
| `EASYTIER_HOSTNAME` | `edge` | 中继在 EasyTier 里显示的 hostname |
| `MAX_FRAME_BYTES` | `1048576` | 单帧上限 |

## 本地开发

```bash
cd cloudflare
pnpm install
cp .dev.vars.example .dev.vars
# 把 .dev.vars 里的密钥换成 `pnpm run keys` 的输出
pnpm run dev
```

本机客户端连 `ws://127.0.0.1:8787/`。

## 可选：自己机器 + Cloudflare Tunnel

如果暂时不想起 Workers / 不想用 `secure-mode`，可以在一台 24 小时在线的 Linux 上跑官方 `easytier-core`，再用 Tunnel 暴露 `ws://127.0.0.1:11011`。见 [docs/tunnel.md](docs/tunnel.md)。不需要 Docker，systemd 也能跑。
