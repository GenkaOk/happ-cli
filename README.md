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
| **Hysteria2** | ✅ | ✅ | UDP/QUIC | TLS, obfs salamander |

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

> `go.mod` declares no `replace` directives, so `go install` is not blocked by
> them: `go install github.com/aimuzov/happ-cli/cmd/happ@latest` works against a
> published tag. A prebuilt binary or a local `go build` is still the simplest
> route.

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
3  hysteria2                hy.example:443       Fast HY2
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
| `--health-check`   | `false` | verify connectivity on start + periodic in every mode; exit on failure |
| `--check-interval`  | `60`    | seconds between health checks |
| `--check-url`       | Cloudflare trace | URL for the startup gate and periodic checks |
| `--dns-proxy`       | `true`  | route DNS through proxy to prevent leaks |
| `--auto-update`     | `false` (`auto-update` in config) | check GitHub for a newer release before connecting (best effort) |
| `--exclude`         | —       | domain or IPv4 CIDR to send direct (repeatable) |
| `--exclude-file`    | `<config dir>/exclude.txt` if present | file with exclusion entries, one per line |
| `--sub`          | active  | subscription name                                   |

The startup gate retries within its 10-second timeout. `--check-url` applies to
both the startup gate and the periodic probe, including `tun-direct`.

In `tun` and `tun-direct` modes the routed-path check resolves the check URL's
host **once, before routes change**, and then probes the resolved IPv4 address
with the original Host/SNI preserved. A router whose DNS stops working after the
routes are installed therefore cannot make a working tunnel look dead. Teardown
requires **three consecutive failed probes**; a single blip is tolerated. A DNS
failure during the routed-path check is printed as a warning and never rolls the
routes back.

### Exclusions

Send selected destinations **around** the tunnel (direct) instead of through it:

```sh
sudo happ connect --mode tun --exclude google.com --exclude 10.0.0.0/8
sudo happ connect --mode tun --exclude-file ./my-excludes.txt
```

Entries are the values of `--exclude` plus the lines of the file. One entry per
line; `#` starts a comment (a whole line, or a trailing comment after
whitespace):

```text
google.com              # google.com and all its subdomains go direct
full:example.com        # only example.com, not its subdomains
keyword:google          # substring match on the domain
regexp:^ads\.           # regular expression on the domain
10.0.0.0/8              # an IPv4 network goes direct
192.0.2.5               # a bare IPv4 address, treated as /32
```

Bare names become `domain:` rules (domain plus every subdomain). Blank lines,
comments and Windows line endings are handled. IPv6 entries are rejected.
Invalid entries are reported as `warning: ignoring exclude entry "..."` on
stderr and skipped; the connection still runs.

Where the file lives: `--exclude-file` if given — a missing explicit file is an
error; otherwise `exclude.txt` next to `config.yaml` is loaded automatically
when it exists. The same values can be set in `config.yaml`:

```yaml
connect:
  exclude:
    - google.com
  exclude-file: ""
```

How it works: xray gets `routing.rules` sending matching traffic to the `direct`
outbound. In `--mode tun` on Linux the direct outbound also carries
`streamSettings.sockopt.mark`, and the tunnel adds
`ip rule add fwmark <mark> lookup main pref 80` so those direct connections
bypass the TUN device instead of looping back into it. Excluded networks get an
additional kernel rule (`ip rule add to <cidr> lookup main pref 85..89`, up to
five networks).

Limitations:

- **Domain matching relies on sniffing** (TLS SNI / HTTP Host). ECH/ESNI or
  non-sniffable traffic cannot be matched by name — only the IP path applies,
  and a domain whose IP is not excluded still goes through the tunnel.
- **Kernel-level exclusion is Linux `--mode tun` only.** On macOS the flags are
  accepted but there is no kernel bypass, so in `--mode tun` excluded traffic
  may still be pulled into the utun; use proxy mode there. In proxy and
  `tun-direct` modes only xray's routing rules apply.
- **DNS lookups still go through the tunnel** unless `--dns-proxy=false`: xray
  queries 1.1.1.1/8.8.8.8 via the proxy outbound.

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
happ update                  # self-update from GitHub releases
happ system-proxy off        # emergency reset of the system proxy (sudo)
```

### Self-update

`happ update` installs the latest [GitHub release](https://github.com/GenkaOk/happ-cli/releases):

```sh
happ update              # check, download, verify, replace the running binary
happ update --check      # report current/latest only; downloads nothing
sudo happ update         # system-wide install (router, /usr/local/bin)
```

The command prints the current and latest versions, picks the asset for the
running platform (`.tar.gz` everywhere except Windows `.zip`; Linux `arm` maps
to `armv5`/`armv6`/`armv7`, and the `mips`/`mipsle` builds are softfloat), and
refuses to replace anything unless the archive's SHA-256 matches the published
`<asset>.sha256` file. Downloads are HTTPS-only and size-capped.

The previous binary is kept as `<exe>.old`; the new one is written to
`<exe>.new` and atomically renamed into place (on Windows the running image is
moved aside first). A symlinked launcher updates the real target, not the link.

A running process keeps using the old build, so restart `happ connect` (or the
service supervising it) to apply the update. Updating a binary owned by root —
on a router, under `/opt/bin`, in `/usr/local/bin` — needs root; when the
target directory is not writable `happ update` names it and refuses.

`connect --auto-update` (config key `auto-update: true`) runs the same check
right before connecting, with a 15-second budget:

```yaml
connect:
  auto-update: true
```

Failures are only printed as `warning: auto-update: ...` and never block the
connection. When it installs a new release it prints `updated to <tag>; this
process keeps running the old build, restart to apply`. Auto-update never
installs over a `dev` build.

Released binaries report their tag as the version (`happ --version`), which is
what the updater compares; locally built binaries still report `dev` and need
`happ update --force` to install a release over them. `--repo owner/name` or
`HAPP_UPDATE_REPO` selects a fork or mirror. `--check` never writes to disk.

## Configuration & storage

State (subscriptions and cached links) is stored as `state.json` in the
per-user config directory (`~/Library/Application Support/happ-cli` on macOS),
overridable with the global `--home` flag.

Values under `connect:` in `config.yaml` act as defaults for the corresponding
`connect` flags; CLI flags override them.

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
3. **routes are installed only after connectivity is verified** — with
   `--health-check` the startup gate runs over the SOCKS proxy before any route
   changes, then the routed path is verified after routes are installed and
   rolled back if it does not pass traffic (the probe dials the IP resolved
   before the routes changed, so router DNS cannot fail it). Without
   `--health-check` there is no gate, so routes are installed immediately;
4. **directly connected subnets stay on the main table** — every prefix from
   `ip -4 route show proto kernel` (except the TUN device), plus
   `169.254.0.0/16` and `224.0.0.0/4`, gets a policy rule (`pref 90..`) pointing
   at `main`, ahead of the tunnel rule at `pref 32760`. Replies to LAN clients,
   SSH to the router and its management UI therefore keep working even while the
   tunnel is down. These prefixes are **not** copied into table 100 via the WAN
   gateway (doing that sent LAN traffic to the ISP and broke LAN↔LAN);
5. the default route is overridden with two `/1` routes scoped to the TUN device;
6. use `--no-routing` to skip route manipulation (only create the TUN device),
   useful on routers where routes are managed externally.

With exclusions configured (`--exclude`, `--exclude-file`, or the auto-loaded
`exclude.txt`), one `ip rule fwmark <mark> lookup main pref 80` rule sends
xray's marked direct connections around the tunnel, and each excluded network
gets `ip rule to <cidr> lookup main` at `pref 85..89` (at most five; additional
networks are skipped with a warning). `cleanupStaleTun` and the tunnel teardown
remove prefs 80 and 85..89 along with the LAN and tunnel rules.

### Troubleshooting: LAN DNS stops working in `-m tun`

If name resolution breaks on LAN clients while the tunnel still passes traffic,
the router's upstream resolver is probably unreachable through the tunnel: a
private ISP DNS server routed into the VPN cannot answer. Point dnsmasq at a
public resolver (for example `1.1.1.1`) or add that server with `--exclude`.
happ does not intercept system DNS itself.

## Limitations

- **TLS `insecure=1` is unsupported by xray-core** (removed 2026-06-01): for a self-signed server, add `pinSHA256`/`pcs` (hex sha256 of the leaf cert) to the link, otherwise the connection is verified normally and will fail.
- **Hysteria2** supports password auth, TLS, and `obfs=salamander`; port hopping (`udphop`) and brutal congestion control are not wired yet.
- **`connect` exits non-zero (1) when a health check tears the tunnel down** (after three consecutive failed probes); Ctrl+C exits 0.
- **`--system-proxy` is macOS-only** (uses `networksetup`).
- **IPv6 is blocked in TUN/TUN-direct mode** (the proxy path is IPv4); IPv6-only destinations become unreachable while connected.
- **Exclusions match domains only with sniffable traffic**: domain rules use the TLS SNI / HTTP Host seen by the inbound, so ECH/ESNI or non-sniffable traffic falls back to the IP path. IPv6 exclude entries are not supported.
- **Kernel-level exclusion is Linux `--mode tun` only**; on macOS only xray's routing rules apply, so excluded traffic can still be pulled into the utun in `--mode tun` (use proxy mode there).
- **DNS still goes through the tunnel** unless `--dns-proxy=false`.
- **`tun-direct` needs manual routing on Linux**: xray-core's TUN only creates the network interface (see `proxy/tun/tun_linux.go`: "nothing more than create a network interface") and installs **no** system routes. Unlike `--mode tun`, it does not redirect traffic by itself — add routes/policy rules yourself, or use `--mode tun`.
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

> `gvisor.dev/gvisor` is a transitive dependency of tun2socks, and tun2socks
> (>= v2.7.0) pulls a snapshot that xray-core also builds against, so no
> `replace` pin is needed. Keep the version Go's minimal version selection picks:
> the 2026-11-12 gvisor snapshot does not compile at all (`pkg/tcpip/stack`
> ships files in two packages), so do not chase `@latest` for gvisor.

### Releasing

Releases are built by [GoReleaser](https://goreleaser.com) in CI on a tag push:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `build-release` workflow (`.github/workflows/build-release.yml`) builds
binaries for 12 OS/arch combinations (linux/darwin/windows, amd64/arm64/armv5-7/mips/mipsle)
and uploads them to GitHub Releases. Tag builds get
`-ldflags "-s -w -X main.version=${GITHUB_REF_NAME}"`, so a released binary
reports its tag as the version (required by `happ update`); branch builds keep
`dev`. Building there honors the `go.mod` `replace` directive (happ-cli is the
main module). Dry-run locally with `goreleaser release --clean --snapshot`.
