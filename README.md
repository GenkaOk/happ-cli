<p align="center">
  <img src="assets/banner.png" alt="happ-cli" width="800" />
</p>

# happ-cli

**English** | [Русский](README.ru.md)

A terminal VPN client compatible with [HAPP](https://happ.su) subscription
profiles. It fetches a subscription, parses its share links
(VLESS / VMess / Trojan / Shadowsocks), and connects through an embedded
[xray-core](https://github.com/XTLS/Xray-core) — as a local proxy, a system
proxy, or a full system-wide TUN tunnel.

Single self-contained binary: xray-core and tun2socks are embedded, no external
binaries required.

## Features

| | happ-cli | Other HAPP clients |
|---|:---:|:---:|
| Self-contained binary (no dependencies) | ✅ | ❌ |
| JSON subscription (Incy format) | ✅ | ❌ |
| System-wide TUN VPN | ✅ | ❌ (proxy only) |
| TUN-direct (ICMP/ping works) | ✅ | ❌ |
| Router support (MIPS/ARM, iptables, LAN bypass) | ✅ | ❌ |
| Auto-failover (round-robin through servers) | ✅ | ❌ |
| Health checks with dead-server tracking | ✅ | ❌ |
| YAML config file | ✅ | ❌ |
| DNS leak prevention (optional) | ✅ | ❌ |

## Supported protocols

| Protocol | Parse | Connect | Transports | Security |
|----------|:---:|:---:|------------|----------|
| **VLESS** | ✅ | ✅ | TCP, WS, gRPC, HTTP/2 | Reality, TLS, XTLS Vision |
| **VMess** | ✅ | ✅ | TCP, WS, gRPC, HTTP/2 | TLS |
| **Trojan** | ✅ | ✅ | TCP, WS | TLS |
| **Shadowsocks** | ✅ | ✅ | TCP | AEAD ciphers |
| **Hysteria2** | ✅ | ❌ | — | — |

## Connection modes

| Mode | Root | ICMP | DNS proxy | Use case |
|------|:---:|:---:|:---:|----------|
| `proxy` | ❌ | ❌ | ✅ | Browser/CLI via SOCKS5 |
| `proxy --system-proxy` | sudo | ❌ | ✅ | macOS system-wide (coexists with VPN) |
| `tun` | sudo | ❌ | ✅ | Full system VPN via tun2socks |
| **`tun-direct`** | sudo | ✅ | ✅ | Full VPN with ICMP (xray TUN, no SOCKS) |

## How it works

```
subscription URL
      │  profile.Fetch (INCY headers)
      ▼
base64 or JSON list ──► link.Parse / json.go ──► []link.Server
                                            │ xray.BuildConfig
                                            ▼
                                    xray-core config (JSON)
                                            │ xray.Start (embedded core)
              ┌─────────────────────────────┼─────────────────────────────┐
              ▼                             ▼                             ▼
      proxy: SOCKS5/HTTP          --system-proxy: networksetup     tun: tun2socks
      on 127.0.0.1                sets system SOCKS/HTTP            + route table
      (no root)                   (sudo)                           (sudo, utun)
```

## Install

### mise (recommended)

Prebuilt binaries are published to GitHub Releases. Install with
[mise](https://mise.jdx.dev) — no Go toolchain required. The binary inside the
archive is `happ` (not `happ-cli`), so pass `exe=happ`:

```sh
mise use -g "github:aimuzov/happ-cli[exe=happ]@latest"
```

or pin it in `mise.toml`:

```toml
[tools]
"github:aimuzov/happ-cli" = { version = "latest", exe = "happ" }
```

The `ubi` backend works the same way against the same releases, if you prefer it:
`ubi:aimuzov/happ-cli[exe=happ]`.

> For frequent installs, set `MISE_GITHUB_TOKEN` (or `GITHUB_TOKEN`) to avoid
> GitHub API rate limits.

### Manual download

Download the archive for your OS/arch from the
[Releases](https://github.com/aimuzov/happ-cli/releases) page, extract it, and
put the `happ` binary on your `PATH`.

| System | Architecture | Download |
|--------|-------------|----------|
| **Linux** | amd64 | [`happ-linux-amd64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-amd64.tar.gz) |
| | arm64 | [`happ-linux-arm64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-arm64.tar.gz) |
| | armv5 | [`happ-linux-armv5.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-armv5.tar.gz) |
| | armv6 | [`happ-linux-armv6.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-armv6.tar.gz) |
| | armv7 | [`happ-linux-armv7.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-armv7.tar.gz) |
| | mips (softfloat) | [`happ-linux-mips.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-mips.tar.gz) |
| | mipsle (softfloat) | [`happ-linux-mipsle.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-mipsle.tar.gz) |
| **macOS** | amd64 (Intel) | [`happ-darwin-amd64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-darwin-amd64.tar.gz) |
| | arm64 (Apple Silicon) | [`happ-darwin-arm64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-darwin-arm64.tar.gz) |
| **Windows** | amd64 | [`happ-windows-amd64.zip`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-windows-amd64.zip) |
| | 386 | [`happ-windows-386.zip`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-windows-386.zip) |

### From source

```sh
git clone https://github.com/aimuzov/happ-cli
cd happ-cli
go build -o happ ./cmd/happ/   # requires Go 1.26+
```

The resulting `happ` binary is self-contained.

> **`go install github.com/aimuzov/happ-cli@latest` does not work.** The build
> relies on a `replace` directive in `go.mod` (to reconcile xray-core and
> tun2socks on gvisor), and `go install pkg@version` ignores `replace`. Use a
> prebuilt binary or clone and build.

## Usage

### Subscriptions

```sh
happ sub add https://panel.example/sub/TOKEN --name myvpn   # add (becomes active)
happ sub list                                               # list subscriptions
happ sub update [name]                                      # re-fetch (default: active)
happ sub use <name>                                         # set the active subscription
happ sub rm <name>                                          # remove
```

`sub list` shows traffic and expiry from the subscription headers:

```
ACTIVE  NAME    TITLE       SERVERS  TRAFFIC          EXPIRES
*       myvpn   My VPN      12       12.4 GB / 200 GB  2026-09-01
```

### Servers

```sh
happ list           # servers in the active subscription
happ list --sub x   # servers in a specific subscription
```

```
#  PROTOCOL                 ADDRESS              TAG
1  vless                    de.example:443       🇩🇪 Germany
2  trojan                   nl.example:443       🇳🇱 Netherlands
3  hysteria2 (unsupported)  hy.example:443       Fast HY2
```

### Connecting

`connect` runs in the foreground until interrupted with `Ctrl+C`. The `selector`
picks a server: empty = first, a number = 1-based index from `happ list`, or a
case-insensitive substring of the server tag.

```sh
happ connect                 # first server, proxy mode
happ connect 2               # server #2
happ connect germany         # first server whose tag matches "germany"

sudo happ connect 1 --system-proxy   # browsers/apps via system proxy (no routing changes)
sudo happ connect 1 --mode tun       # full system-wide VPN
```

In plain proxy mode, point apps at `socks5://127.0.0.1:10808` (Firefox: enable
"Proxy DNS when using SOCKS v5").

### `connect` flags

| Flag             | Default | Description                                         |
| ---------------- | ------- | --------------------------------------------------- |
| `-m, --mode`     | `proxy` | `proxy` or `tun`                                    |
| `--socks`        | `10808` | local SOCKS5 port                                   |
| `--http`         | `10809` | local HTTP proxy port (proxy mode)                  |
| `--system-proxy` | `false` | set the macOS system proxy (proxy mode, needs sudo) |
| `--no-routing`    | `false` | create TUN device without modifying routes |
| `--skip-firewall`  | `false` | skip iptables rules (tun/tun-direct) |
| `--health-check`   | `false` | verify connectivity on start + periodic; exit on failure |
| `--check-interval`  | `60`    | seconds between health checks |
| `--check-url`       | Cloudflare trace | URL for health check |
| `--dns-proxy`       | `true`  | route DNS through proxy to prevent leaks |
| `--sub`          | active  | subscription name                                   |

### Three ways to route traffic, compared

- **`connect` (proxy)** — only apps explicitly pointed at
  `socks5://127.0.0.1:10808` (e.g. Firefox with remote DNS). No root.
- **`connect --system-proxy`** — sets, on every enabled network service, the
  system SOCKS (`--socks` port) and HTTP/HTTPS (`--http` port) proxies, so
  Safari/Chrome and apps that ignore SOCKS go through the proxy. Does **not**
  touch the routing table, so it **coexists with another active VPN**. Needs
  `sudo`; the previous proxy settings are restored on exit. If a session was
  killed (`kill -9`) and the proxy stuck, reset it with
  `sudo happ system-proxy off`.
- **`connect --mode tun`** — a full system VPN via a utun device; captures all
  traffic. Needs `sudo`. If another VPN is active at the same time, disconnect it
  first so the tunnels don't fight over routes/DNS.

### Other commands

```sh
happ config [selector]       # print the generated xray-core config (debug)
happ system-proxy off        # emergency reset of the system proxy (sudo)
```

## Configuration & storage

State (subscriptions and cached links) is stored as `state.json` in the
per-user config directory (`~/Library/Application Support/happ-cli` on macOS),
overridable with the global `--home` flag.

## TUN mode details

### macOS

1. the server address is resolved to IP(s), and a host route to each is pinned to
   its current next hop (a physical gateway, or an already-active VPN interface),
   so the proxy's own connection to the server does not loop back into the tunnel;
2. a `utun` device is created and tun2socks forwards its traffic to the local
   SOCKS proxy served by xray;
3. the default route is overridden with two `/1` routes scoped to the utun device;
4. global IPv6 is routed into `lo0` (blocked);
5. on `Ctrl+C` all routes are removed in reverse order.

### Linux

1. server IPs are pinned to their current next hop (same as macOS);
2. a TUN device is created via `/dev/net/tun`;
3. **local subnets are preserved** — `192.168.0.0/16`, `172.16.0.0/12`,
   `10.0.0.0/8`, link-local, and multicast stay on the physical interface so
   LAN and router management remain reachable;
4. the default route is overridden with two `/1` routes scoped to the TUN device;
5. use `--no-routing` to skip route manipulation (only create the TUN device),
   useful on routers where routes are managed externally.

## Limitations

- **Hysteria2** servers are parsed and listed but cannot be connected (xray-core has no Hysteria2 outbound).
- **`--system-proxy` is macOS-only** (uses `networksetup`).
- **IPv6 is blocked in TUN/TUN-direct mode** (the proxy path is IPv4); IPv6-only destinations become unreachable while connected.
- `connect` runs in the **foreground**; there is no background daemon yet.
- A `kill -9` skips cleanup, but stale routes are auto-cleaned on the next `connect`. Use `sudo happ cleanup-tun` for manual recovery.

## Project layout

| Package             | Responsibility                                                    |
| ------------------- | ----------------------------------------------------------------- |
| `cmd/happ`          | entry point                                                       |
| `internal/check`    | connectivity health checks (Cloudflare trace)                     |
| `internal/cli`      | cobra commands                                                    |
| `internal/config`   | YAML configuration file                                           |
| `internal/device`   | per-machine device identity (HWID + UUID)                         |
| `internal/firewall` | iptables FORWARD rules (Linux)                                    |
| `internal/link`     | parse share links (vless/vmess/trojan/ss/hysteria2)               |
| `internal/profile`  | fetch a subscription, decode base64/JSON body + headers           |
| `internal/store`    | persist subscriptions, cached links, last-used tracking           |
| `internal/tunnel`   | TUN mode: tun2socks + route management (macOS + Linux)            |
| `internal/sysproxy` | macOS system proxy via networksetup                               |
| `internal/xray`     | build xray-core config from a server, run the embedded core       |

## Development

```sh
go test ./...        # unit tests + a real end-to-end proxy test
go vet ./...
```

The xray integration test starts a real Shadowsocks server and a client built
from a `link.Server`, then verifies an HTTP request routed through the client's
SOCKS inbound reaches a target through the proxy.

> xray-core and tun2socks require different `gvisor.dev/gvisor` versions; a
> `replace` directive in `go.mod` pins gvisor to the version both build against.
> Don't drop it — see the comment there.

### Releasing

Releases are built by [GoReleaser](https://goreleaser.com) in CI on a tag push:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `build-release` workflow (`.github/workflows/build-release.yml`) builds
binaries for 12 OS/arch combinations (linux/darwin/windows, amd64/arm64/armv5-7/mips/mipsle)
and uploads them to GitHub Releases. Building there
honors the `go.mod` `replace` directive (happ-cli is the main module). Dry-run
locally with `goreleaser release --clean --snapshot`.
