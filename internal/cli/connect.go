package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/check"
	"github.com/aimuzov/happ-cli/internal/config"
	"github.com/aimuzov/happ-cli/internal/firewall"
	"github.com/aimuzov/happ-cli/internal/link"
	"github.com/aimuzov/happ-cli/internal/sysproxy"
	"github.com/aimuzov/happ-cli/internal/tunnel"
	"github.com/aimuzov/happ-cli/internal/xray"
)

// directSOMark is the SO_MARK carried by xray's direct outbound in Linux TUN
// mode. The tunnel installs a matching `ip rule fwmark <mark> lookup main`, so
// excluded traffic leaves directly instead of looping back into the TUN.
const directSOMark = 255

// excludeFileName is auto-loaded from the config directory when it exists and
// no --exclude-file was given.
const excludeFileName = "exclude.txt"

// connectOpts groups all connection flags into a single struct.
type connectOpts struct {
	socksPort     int
	httpPort      int
	systemProxy   bool
	noRouting     bool
	skipFirewall  bool
	healthCheck   bool
	checkInterval int    // seconds, default 60
	checkURL      string // URL for health check, default Cloudflare trace
	dnsProxy      bool
	exclude       []string // raw --exclude values
	excludeFile   string   // --exclude-file path

	// Resolved from exclude/excludeFile at connect time.
	excludeEntries []string
	directMark     int
	directCIDRs    []string
}

func newConnectCmd(deps *Deps) *cobra.Command {
	var mode, subName string
	var opts connectOpts

	// Load config defaults — CLI flags override. Each flag is registered with
	// the loaded value as its default; an explicitly passed flag still wins
	// (cobra/pflag keeps the parsed value over the registered default).
	dir, _ := storeDir()
	cfg, _ := config.Load(dir)
	if cfg == nil {
		cfg = config.Default()
	}
	opts.socksPort = cfg.Connect.SocksPort
	opts.httpPort = cfg.Connect.HTTPPort
	opts.systemProxy = cfg.Connect.SystemProxy
	opts.noRouting = cfg.Connect.NoRouting
	opts.skipFirewall = cfg.Connect.SkipFirewall
	opts.healthCheck = cfg.Connect.HealthCheck
	opts.checkInterval = cfg.Connect.CheckInterval
	opts.checkURL = cfg.Connect.CheckURL
	opts.dnsProxy = cfg.Connect.DNSProxy
	opts.exclude = cfg.Connect.Exclude
	opts.excludeFile = cfg.Connect.ExcludeFile
	mode = cfg.Connect.Mode

	cmd := &cobra.Command{
		Use:     "connect [selector]",
		Aliases: []string{"up"},
		Short:   "Connect to a server (proxy or full TUN tunnel)",
		Long: "Connect runs in the foreground until interrupted (Ctrl+C).\n\n" +
			"selector picks the server: empty = first, a number = 1-based index,\n" +
			"or a case-insensitive substring of the server tag.\n\n" +
			"Modes:\n" +
			"  proxy     local SOCKS5 + HTTP proxy on 127.0.0.1 (no root)\n" +
			"  tun       system-wide VPN via tun2socks (requires root, macOS/Linux)\n" +
			"  tun-direct  system-wide VPN via xray TUN, ICMP supported (requires root)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeServerSelector(deps),
		RunE: func(cmd *cobra.Command, args []string) error {
			sub, err := resolveSub(deps.Store, subName)
			if err != nil {
				return err
			}

			servers := sub.Servers()
			if len(servers) == 0 {
				return fmt.Errorf("subscription has no servers")
			}

			// Load last-used tracking to pick the next server in round-robin.
			_ = loadLastUsed(deps)

			selector := ""
			if len(args) > 0 {
				selector = args[0]
			}

			var srv *link.Server
			var idx int
			if selector == "" && deps.LastUsed != nil {
				srv, idx = deps.LastUsed.NextServer(servers)
			} else {
				srv, idx, err = selectServer(servers, selector)
				if err != nil {
					return err
				}
			}

			if !xray.Supported(srv.Protocol) {
				return fmt.Errorf("server #%d uses %q, which xray-core cannot dial; pick another server", idx+1, srv.Protocol)
			}

			fmt.Printf("Server #%d: %s [%s] %s:%d\n", idx+1, srv.Tag, srv.Protocol, srv.Address, srv.Port)

			if srv.AllowInsecure && srv.PinSHA256 == "" {
				fmt.Fprintf(os.Stderr, "warning: server #%d sets insecure=1, but xray-core no longer supports skipping TLS verification; connecting with normal verification (add pinSHA256=<hex sha256 of the server cert> to the link for a self-signed server)\n", idx+1)
			}

			// Resolve exclusions (inline values + file). Broken entries are
			// warnings, not fatal: one typo must not block the connection.
			dir, err := storeDir()
			if err != nil {
				return err
			}
			entries, err := loadExcludeEntries(opts.exclude, opts.excludeFile, dir)
			if err != nil {
				return err
			}
			opts.excludeEntries = entries
			exclusions, excludeErrs := xray.ParseExclusions(entries)
			for _, e := range excludeErrs {
				fmt.Fprintf(os.Stderr, "warning: ignoring exclude entry %v\n", e)
			}
			if len(exclusions) > 0 {
				fmt.Printf("Exclusions: %d (domains: %d, networks: %d)\n",
					len(exclusions), len(xray.ExcludedDomains(exclusions)), len(xray.ExcludedCIDRs(exclusions)))
			}
			// Kernel-level bypass (SO_MARK rule + per-network ip rules) exists
			// only for the Linux tun2socks tunnel; elsewhere xray's routing
			// rules alone decide what goes direct.
			if mode == "tun" && runtime.GOOS == "linux" {
				opts.directMark = directSOMark
				opts.directCIDRs = xray.ExcludedCIDRs(exclusions)
			}

			var connErr error
			switch mode {
			case "proxy":
				connErr = runProxy(cmd.Context(), srv, opts)
			case "tun":
				connErr = runTun(cmd.Context(), srv, opts)
			case "tun-direct":
				connErr = runTunDirect(cmd.Context(), srv, opts)
			default:
				return fmt.Errorf("unknown mode %q (use 'proxy', 'tun', or 'tun-direct')", mode)
			}

			// Remember which server was tried, so the next run picks the next one.
			if deps.LastUsed != nil {
				_ = deps.LastUsed.Mark(srv)
			}
			return connErr
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", mode, "connection mode: proxy or tun")
	cmd.Flags().IntVar(&opts.socksPort, "socks", opts.socksPort, "local SOCKS5 port")
	cmd.Flags().IntVar(&opts.httpPort, "http", opts.httpPort, "local HTTP proxy port (proxy mode)")
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	cmd.Flags().BoolVar(&opts.systemProxy, "system-proxy", opts.systemProxy, "set the macOS system SOCKS proxy (requires sudo, proxy mode)")
	cmd.Flags().BoolVar(&opts.noRouting, "no-routing", opts.noRouting, "create TUN device without modifying routes (tun/tun-direct)")
	cmd.Flags().BoolVar(&opts.skipFirewall, "skip-firewall", opts.skipFirewall, "skip iptables rules (tun/tun-direct)")
	cmd.Flags().BoolVar(&opts.healthCheck, "health-check", opts.healthCheck, "verify connectivity; exit on failure")
	cmd.Flags().IntVar(&opts.checkInterval, "check-interval", opts.checkInterval, "seconds between health checks")
	cmd.Flags().StringVar(&opts.checkURL, "check-url", opts.checkURL, "URL for health check")
	cmd.Flags().BoolVar(&opts.dnsProxy, "dns-proxy", opts.dnsProxy, "route DNS through proxy (disable for local DNS)")
	cmd.Flags().StringArrayVar(&opts.exclude, "exclude", opts.exclude, "domain or IPv4 CIDR to send outside the tunnel (repeatable)")
	cmd.Flags().StringVar(&opts.excludeFile, "exclude-file", opts.excludeFile, "file with exclusion entries, one per line (default: <config dir>/exclude.txt if present)")
	_ = cmd.RegisterFlagCompletionFunc("sub", completeSubFlag(deps))
	_ = cmd.RegisterFlagCompletionFunc("mode", func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return []cobra.Completion{
			cobra.CompletionWithDesc("proxy", "local SOCKS5 + HTTP proxy (no root)"),
			cobra.CompletionWithDesc("tun", "system-wide VPN via tun2socks (sudo)"),
			cobra.CompletionWithDesc("tun-direct", "system-wide VPN via xray TUN, ICMP OK (sudo)"),
		}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// loadExcludeEntries combines inline exclude values with the lines of the
// exclude file. An explicitly given file must be readable; the auto-loaded
// <config dir>/exclude.txt is optional and silently skipped when absent.
// Blank lines and # comments are handled later by xray.ParseExclusions.
func loadExcludeEntries(inline []string, file, dir string) ([]string, error) {
	entries := append([]string(nil), inline...)

	path := file
	explicit := path != ""
	if !explicit {
		path = filepath.Join(dir, excludeFileName)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicit {
			return nil, fmt.Errorf("read exclude file %s: %w", path, err)
		}
		// The implicit file is a convenience: absent means "no exclusions".
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: cannot read exclude file %s: %v\n", path, err)
		}
		return entries, nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		entries = append(entries, line)
	}
	return entries, nil
}

func runProxy(ctx context.Context, srv *link.Server, o connectOpts) error {
	cfg, err := xray.BuildConfig(srv, xray.Options{
		SocksPort: o.socksPort,
		HTTPPort:  o.httpPort,
		DNSProxy:  o.dnsProxy,
		Exclude:   o.excludeEntries,
	})
	if err != nil {
		return err
	}
	raw, err := cfg.JSON()
	if err != nil {
		return err
	}
	inst, err := xray.Start(raw)
	if err != nil {
		return err
	}
	defer inst.Close()

	fmt.Printf("Proxy is up:\n  SOCKS5  127.0.0.1:%d\n  HTTP    127.0.0.1:%d\n", o.socksPort, o.httpPort)

	// Health checks: startup gate plus periodic verification.
	socksAddr := fmt.Sprintf("127.0.0.1:%d", o.socksPort)
	probe := func() error {
		_, err := check.FetchURL(socksAddr, o.checkURL, 5*time.Second)
		return err
	}
	ctx, cancelHealth := startHealthCheck(ctx, o.checkInterval, o.healthCheck, probe)
	defer cancelHealth(nil)

	if o.healthCheck {
		if _, err := check.WaitURL(o.socksPort, o.checkURL, 10*time.Second); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
	} else {
		go check.PrintIP(o.socksPort)
	}

	if o.systemProxy {
		restore, err := sysproxy.Enable("127.0.0.1", o.socksPort, o.httpPort)
		if err != nil {
			return fmt.Errorf("enable system proxy: %w", err)
		}
		defer func() {
			if err := restore(); err != nil {
				fmt.Println("warning: failed to restore system proxy:", err)
			}
		}()
		fmt.Println("System SOCKS/HTTP proxy set on all network services (will be restored on exit).")
	}

	fmt.Println("Press Ctrl+C to disconnect.")
	<-ctx.Done()
	if errors.Is(context.Cause(ctx), errHealthCheckFailed) {
		fmt.Println("\nHealth check failed — disconnecting.")
	}
	fmt.Println("\nDisconnecting...")
	return disconnectReason(ctx)
}

func runTun(ctx context.Context, srv *link.Server, o connectOpts) error {
	ips, err := resolveIPv4(srv.Address)
	if err != nil {
		return fmt.Errorf("resolve server address %q: %w", srv.Address, err)
	}

	// Pin the outbound to a concrete IP and keep the TLS SNI on the domain, so
	// xray dials the exact IP we route around the tunnel.
	pinned := *srv
	if pinned.SNI == "" {
		pinned.SNI = srv.Address
	}
	pinned.Address = ips[0]

	cfg, err := xray.BuildConfig(&pinned, xray.Options{
		SocksPort:  o.socksPort,
		DNSProxy:   o.dnsProxy,
		Exclude:    o.excludeEntries,
		DirectMark: o.directMark,
	})
	if err != nil {
		return err
	}
	raw, err := cfg.JSON()
	if err != nil {
		return err
	}
	inst, err := xray.Start(raw)
	if err != nil {
		return err
	}
	defer inst.Close()

	// Verify the proxy path BEFORE touching system routes. If the server is
	// unreachable, the router keeps normal routing and management stays up.
	// Routes are installed only after this gate passes, and rolled back by the
	// tun.Close() defer below if anything later fails.
	if o.healthCheck {
		if _, err := check.WaitURL(o.socksPort, o.checkURL, 10*time.Second); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
	} else {
		go check.PrintIP(o.socksPort)
	}

	tun, err := tunnel.Start(tunnel.Options{
		SocksAddr:   fmt.Sprintf("127.0.0.1:%d", o.socksPort),
		ServerIPs:   ips,
		SkipRoutes:  o.noRouting,
		NoFirewall:  o.skipFirewall,
		DirectMark:  o.directMark,
		DirectCIDRs: o.directCIDRs,
	})
	if err != nil {
		return err
	}
	defer tun.Close()

	fmt.Printf("TUN tunnel is up; all traffic is routed through %s.\n", srv.Tag)

	// Health checks: startup gate already passed over SOCKS; the periodic probe
	// must test the ROUTED path (direct), not the SOCKS path, otherwise a dead
	// tun2socks or broken routes would still look healthy.
	probe := func() error {
		_, err := check.FetchDirect(o.checkURL, 5*time.Second)
		return err
	}
	ctx, cancelHealth := startHealthCheck(ctx, o.checkInterval, o.healthCheck, probe)
	defer cancelHealth(nil)

	// Routes are now installed, so the routed path is finally testable. On
	// failure the deferred tun.Close() rolls the routes back before returning.
	if o.healthCheck {
		if _, err := check.WaitDirect(o.checkURL, 5*time.Second); err != nil {
			return fmt.Errorf("tunnel does not pass traffic (routes rolled back): %w", err)
		}
	}

	fmt.Println("Press Ctrl+C to disconnect and restore routing.")
	<-ctx.Done()
	if errors.Is(context.Cause(ctx), errHealthCheckFailed) {
		fmt.Println("\nHealth check failed — disconnecting.")
	}
	fmt.Println("\nDisconnecting and restoring routes...")
	return disconnectReason(ctx)
}

// runTunDirect uses xray's built-in TUN inbound — no tun2socks, no SOCKS.
// Xray creates the TUN device and handles routing. ICMP/ping works.
func runTunDirect(ctx context.Context, srv *link.Server, o connectOpts) error {
	cfg, err := xray.BuildConfig(srv, xray.Options{TUNDirect: true, DNSProxy: o.dnsProxy, Exclude: o.excludeEntries})
	if err != nil {
		return err
	}
	raw, err := cfg.JSON()
	if err != nil {
		return err
	}
	inst, err := xray.Start(raw)
	if err != nil {
		return err
	}
	defer inst.Close()

	// Apply firewall rules for xray's TUN device.
	if !o.skipFirewall {
		if iface := findTunIface(); iface != "" {
			cleanupFW := firewall.Rules(iface)
			defer cleanupFW()
		}
	}

	fmt.Println("TUN-direct is up (ICMP supported).")

	// Health checks run over the TUN's system route (there is no SOCKS inbound).
	probe := func() error {
		_, err := check.FetchDirect(o.checkURL, 5*time.Second)
		return err
	}
	ctx, cancelHealth := startHealthCheck(ctx, o.checkInterval, o.healthCheck, probe)
	defer cancelHealth(nil)

	if o.healthCheck {
		if _, err := check.WaitDirect(o.checkURL, 10*time.Second); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
	}

	fmt.Println("Press Ctrl+C to disconnect.")
	<-ctx.Done()
	if errors.Is(context.Cause(ctx), errHealthCheckFailed) {
		fmt.Println("\nHealth check failed — disconnecting.")
	}
	fmt.Println("\nDisconnecting...")
	return disconnectReason(ctx)
}

// findTunIface finds a TUN interface created by xray (usually tun0).
func findTunIface() string {
	out, err := exec.Command("ip", "-br", "link", "show", "type", "tun").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "tun") {
			return strings.Fields(line)[0]
		}
	}
	return ""
}

// checkExternalHTTP fetches a URL through the system's default route to verify
// the VPN is passing traffic, and returns the exit IP from a Cloudflare-trace
// body. It bypasses environment proxies via check.FetchDirect.
func checkExternalHTTP(urlStr string, timeout time.Duration) (string, error) {
	body, err := check.FetchDirect(urlStr, timeout)
	if err != nil {
		return "", err
	}
	// FetchDirect already extracts the ip= field from a Cloudflare-style body;
	// accept that value directly, otherwise scan a raw body for ip=.
	if ip := net.ParseIP(strings.TrimSpace(body)); ip != nil {
		fmt.Printf("  Exit IP: %s\n", strings.TrimSpace(body))
		return strings.TrimSpace(body), nil
	}
	for _, line := range strings.Split(body, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && k == "ip" {
			fmt.Printf("  Exit IP: %s\n", v)
			return v, nil
		}
	}
	return "", fmt.Errorf("ip not found")
}

// errHealthCheckFailed is the cancellation cause used when a health probe
// fails, so the connect loops can tell it apart from Ctrl+C or a parent cancel.
var errHealthCheckFailed = errors.New("health check failed")

// errTunnelLost is returned by the connect loops when the tunnel was torn down
// because connectivity stopped working. A non-zero exit lets supervisors
// (procd, the Entware init script, systemd) tell a real failure from Ctrl+C.
var errTunnelLost = errors.New("connection lost: health check failed, tunnel torn down")

// disconnectReason maps the context's cancellation cause to the error the
// connect loops should return. A user-initiated stop returns nil (exit 0).
func disconnectReason(ctx context.Context) error {
	if errors.Is(context.Cause(ctx), errHealthCheckFailed) {
		return errTunnelLost
	}
	return nil
}

// startHealthCheck runs periodic connectivity checks with probe. When enabled
// is false it is a no-op. Otherwise it ticks every intervalSec seconds (60 when
// < 1) and cancels the returned context with errHealthCheckFailed after the
// first failed probe.
func startHealthCheck(parent context.Context, intervalSec int, enabled bool, probe func() error) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	if !enabled {
		return ctx, cancel
	}
	if intervalSec < 1 {
		intervalSec = 60
	}
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := probe(); err != nil {
					fmt.Printf("\nHealth check failed: %v\n", err)
					cancel(errHealthCheckFailed)
					return
				}
			}
		}
	}()
	return ctx, cancel
}

// resolveIPv4 returns the IPv4 addresses for host, or host itself if it is one.
func resolveIPv4(host string) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return nil, fmt.Errorf("IPv6 server addresses are not supported in TUN mode yet")
		}
		return []string{host}, nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, a := range addrs {
		if v4 := a.To4(); v4 != nil {
			out = append(out, v4.String())
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no IPv4 address found for %q", host)
	}
	return out, nil
}
