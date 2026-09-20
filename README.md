# EasyTier 中心节点（Cloudflare Workers）

不需要 Docker，也不需要自己的 24 小时主机。中心节点直接跑在 Cloudflare 边缘：`wrangler deploy` 之后，客户端用 `wss://你的 Worker 域名:443/` 接入。

```text
远程客户端  --wss://lm191549149.me:443-->  Cloudflare Worker
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

### 绑定自己的域名（国内直连）

当前中心节点自定义域名是 **`lm191549149.me`**，Worker 已经绑好。GUI 初始节点填 `wss://lm191549149.me:443/`。

阿里云 DNS 已经改对。Cloudflare 站点目前仍是 **Pending（未激活）**，解析只有占位 IPv6 `100::`、没有 IPv4，证书也还没签发，所以 `https://lm191549149.me/healthz` 会打不开。

浏览器打开 `https://lm191549149.me/` 会转到博客首页。EasyTier 仍用同一主机的 WebSocket 接入。

## 2. 客户端接入

必须开 `--secure-mode`，网络名/密钥与 Secret 里的 `EASYTIER_NETWORKS` 一致。Worker 只开 **443**。EasyTier GUI 里 WSS 默认端口是 **11012**，不改就会 `connect timeout`。

### easytier-gui

打开 **添加新网络 / 配置网络**，不要选「公共服务器」（那是官方共享节点，连不上这个 Worker）。

| GUI 字段 | 填什么 |
| --- | --- |
| 网络方式 | **手动** |
| 初始节点 | 协议 **wss**，主机 `lm191549149.me`，端口 **443** |
| 或一整条 URI | `wss://lm191549149.me:443/` |
| 网络名称 | 部署时的 `EASYTIER_NETWORK_NAME`（例如 `office`） |
| 网络密码 | 部署时的 `EASYTIER_NETWORK_SECRET` |
| 虚拟IPv4 / DHCP | 打开 **DHCP** |
| 高级设置 → 禁用加密 | **不要勾** |
| 高级设置 → 启用私有模式 | 可关；这不是安全模式 |

保存后再打开配置，确认端口仍是 **443**，不是 `11012`。有的 GUI 会把 443 改回 11012，这时用 **编辑配置文件** 写成：

```toml
[[peer]]
uri = "wss://lm191549149.me:443/"
```

输入初始节点后要点一下列表项确认，确认后地址会变成卡片。然后点 **运行网络**。

连这个 Worker **不需要** WireGuard 入站。GUI 默认会监听 `wg://0.0.0.0:11011`，Windows 上若端口已被另一个 EasyTier 实例占用，会报 `监听器添加失败` / `AddrInUse (10048)`。到 **高级设置 → 监听地址** 删掉 `wg://0.0.0.0:11011` 即可；这条失败一般会 `retry listen later`，TCP/UDP 和去 Worker 的 `wss://` 仍能用。真要留 WG，把端口改成空闲的（例如 `wg://0.0.0.0:11021`），或多开网络时每个实例用不同端口。

**安全模式必须开**。这个 Worker 会直接关掉旧握手，GUI 就会报 `conn closed during wait handshake response`。EasyTier 2.6.x 配置必须是表，**不能**写 `secure_mode = true`（解析会失败）：

```toml
[secure_mode]
enabled = true
local_private_key = "<本机私钥，base64>"
local_public_key = "<本机公钥，base64>"
```

- 新版 GUI：展开 **高级设置 → 功能开关**，打开 **安全模式 / Secure Mode**（在「禁用加密」旁边），不要和「禁用加密」「私有模式」搞混。
- 没有这个开关：点 **编辑配置文件**，贴上上面的 `[secure_mode]` 段。密钥用 `cd cloudflare && pnpm run keys` 生成。只写 `enabled = true` 不写密钥时，2.6.4 会报 `local private key is not set`。
- 需要 **EasyTier 2.6.4+**。更早的 GUI 没有 Noise 握手，连不上这个 Worker。

可选：把部署时打印的中心节点公钥配到 `peer_public_key`，用来锁定 Worker 身份。GUI 里若没有这项，同样写进配置文件。

家宽或公司出口节点再填 **子网代理CIDR**，例如 `192.168.1.0/24`（必须是那台机器已经能直达的网段）。

### 命令行

```bash
easytier-core \
  -d \
  --network-name office \
  --network-secret '和部署时相同的密钥' \
  --secure-mode \
  -p 'wss://lm191549149.me:443/'
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

## 3. 家里访问公司内网

Worker 只做发现和中继。不要一条条加 IP。选下面一种即可。

### 方案 A：出口节点（和公司上网一样）

公司电脑勾选 **启用出口节点**。家里电脑 **出口节点列表** 填公司那台的虚拟 IPv4（节点列表里能看到）。家里 DNS、内网域名、内网 IP 全部走公司，浏览器继续用原来的网址。

公司电脑必须一直开着。家里所有上网也会变慢、并走公司出口，不要在未授权时用。

### 方案 B：一次导出全部内网（外网仍走家里）

公司电脑 **子网代理CIDR** 一次填齐私网（可多条），不要逐个业务 IP：

- `172.16.0.0/12`（覆盖 `172.17.1.145` 这类地址）
- 公司若用 `10.x`，填公司实际的那一段（如 `10.0.0.0/16`），**不要**填整个 `10.0.0.0/8`，以免和 EasyTier 虚拟网（常见 `10.126.126.0/24`）打架
- 公司若用 `192.168.x`，填公司那一段。家里也是 `192.168.x` 时不要填整个 `192.168.0.0/16`，只填公司网段，或写成 `192.168.10.0/24->10.33.10.0/24`

家里电脑不要重复填这些网段。公网已经能解析的内网域名（如 `git.fortunecare.com.cn`）不用改 URL；只有公司 DNS 才有的名字，把家里 EasyTier 网卡或系统 DNS 指到公司 DNS 的内网 IP。

### 方案 C：SOCKS5（只让浏览器走公司，不用加 IP）

SOCKS5 要开在**公司电脑**上。连接从公司这台机器打出去，能打开的内网网站家里浏览器也能开，不必填子网代理。

1. 公司电脑 EasyTier **高级设置 → socks5服务器** 填 `1080`，运行网络。防火墙只允许虚拟网访问 1080，不要对公网开放。
2. 家里电脑加入同一网络，节点列表里记下公司电脑的 **虚拟IPv4**。
3. 家里浏览器走 SOCKS5，并且 **DNS 也走代理**（SOCKS5h），否则域名会在家里解析，还是打不开：
   - Firefox：设置 → 网络设置 → 手动代理 → SOCKS v5 → 主机填公司虚拟 IP、端口 `1080` → 勾选「使用 SOCKS v5 时代理 DNS 查询」
   - Chrome 建议用 SwitchyOmega，协议选 SOCKS5，同样打开「通过代理进行 DNS」
4. 浏览器打开原来的网址即可，例如 `https://git.fortunecare.com.cn/`。不想走内网的网站，给浏览器加直连规则，或关掉代理。

这只影响配了代理的软件。远程桌面、局域网共享仍要用方案 A/B，或继续用公司电脑的虚拟 IP。不要把 SOCKS5 开在家里电脑上当「替代加 IP」——家里的 1080 只能进 EasyTier 虚拟网，进不了公司 `172.17.x`，除非再做子网代理。

### 两台电脑的共同配置

easytier-gui 2.6.4+：手动、`wss://lm191549149.me:443/`、同一网络名和密码、DHCP、安全模式。每台自己的 `[secure_mode]` 密钥。管理员运行。公司防火墙放行 EasyTier。只在你有权访问该内网时使用。

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
