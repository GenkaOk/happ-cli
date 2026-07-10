package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
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
}

func newConnectCmd(deps *Deps) *cobra.Command {
	var mode, subName string
	var opts connectOpts

	// Load config defaults — CLI flags override.
	dir, _ := storeDir()
	cfg, _ := config.Load(dir)
	if cfg != nil {
		opts.socksPort = cfg.Connect.SocksPort
		opts.httpPort = cfg.Connect.HTTPPort
		opts.systemProxy = cfg.Connect.SystemProxy
		opts.noRouting = cfg.Connect.NoRouting
		opts.skipFirewall = cfg.Connect.SkipFirewall
		opts.healthCheck = cfg.Connect.HealthCheck
		opts.checkInterval = cfg.Connect.CheckInterval
		opts.checkURL = cfg.Connect.CheckURL
		opts.dnsProxy = cfg.Connect.DNSProxy
		mode = cfg.Connect.Mode
	}

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
	cmd.Flags().StringVarP(&mode, "mode", "m", "proxy", "connection mode: proxy or tun")
	cmd.Flags().IntVar(&opts.socksPort, "socks", defaultSocksPort, "local SOCKS5 port")
	cmd.Flags().IntVar(&opts.httpPort, "http", defaultHTTPPort, "local HTTP proxy port (proxy mode)")
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	cmd.Flags().BoolVar(&opts.systemProxy, "system-proxy", false, "set the macOS system SOCKS proxy (requires sudo, proxy mode)")
	cmd.Flags().BoolVar(&opts.noRouting, "no-routing", false, "create TUN device without modifying routes (tun/tun-direct)")
	cmd.Flags().BoolVar(&opts.skipFirewall, "skip-firewall", false, "skip iptables rules (tun/tun-direct)")
	cmd.Flags().BoolVar(&opts.healthCheck, "health-check", false, "verify connectivity; exit on failure")
	cmd.Flags().IntVar(&opts.checkInterval, "check-interval", 60, "seconds between health checks")
	cmd.Flags().StringVar(&opts.checkURL, "check-url", "https://cloudflare.com/cdn-cgi/trace", "URL for health check")
	cmd.Flags().BoolVar(&opts.dnsProxy, "dns-proxy", true, "route DNS through proxy (disable for local DNS)")
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

func runProxy(ctx context.Context, srv *link.Server, o connectOpts) error {
	cfg, err := xray.BuildConfig(srv, xray.Options{SocksPort: o.socksPort, HTTPPort: o.httpPort, DNSProxy: o.dnsProxy})
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

	// Health checks: periodic IP verification.
	ctx, cancelHealth := startHealthCheck(ctx, o.socksPort, o.healthCheck, o.checkInterval, o.checkURL)
	defer cancelHealth()

	if o.healthCheck {
		if _, err := check.WaitIP(o.socksPort, 10*time.Second); err != nil {
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
	if ctx.Err() == context.Canceled && o.healthCheck {
		fmt.Println("\nHealth check failed — disconnecting.")
	}
	fmt.Println("\nDisconnecting...")
	return nil
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

	cfg, err := xray.BuildConfig(&pinned, xray.Options{SocksPort: o.socksPort, DNSProxy: o.dnsProxy})
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

	tun, err := tunnel.Start(tunnel.Options{
		SocksAddr:  fmt.Sprintf("127.0.0.1:%d", o.socksPort),
		ServerIPs:  ips,
		SkipRoutes: o.noRouting,
		NoFirewall: o.skipFirewall,
	})
	if err != nil {
		return err
	}
	defer tun.Close()

	fmt.Printf("TUN tunnel is up; all traffic is routed through %s.\n", srv.Tag)

	// Health checks: periodic IP verification.
	ctx, cancelHealth := startHealthCheck(ctx, o.socksPort, o.healthCheck, o.checkInterval, o.checkURL)
	defer cancelHealth()

	if o.healthCheck {
		if _, err := check.WaitIP(o.socksPort, 10*time.Second); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
	} else {
		go check.PrintIP(o.socksPort)
	}

	fmt.Println("Press Ctrl+C to disconnect and restore routing.")
	<-ctx.Done()
	if ctx.Err() == context.Canceled && o.healthCheck {
		fmt.Println("\nHealth check failed — disconnecting.")
	}
	fmt.Println("\nDisconnecting and restoring routes...")
	return nil
}

// runTunDirect uses xray's built-in TUN inbound — no tun2socks, no SOCKS.
// Xray creates the TUN device and handles routing. ICMP/ping works.
func runTunDirect(ctx context.Context, srv *link.Server, o connectOpts) error {
	cfg, err := xray.BuildConfig(srv, xray.Options{TUNDirect: true, DNSProxy: o.dnsProxy})
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

	// Health checks.
	ctx, cancelHealth := startHealthCheck(ctx, 0, false, 0, "") // no SOCKS, no periodic check
	defer cancelHealth()

	if o.healthCheck {
		// Simple HTTP check through system route (which goes via TUN).
		if _, err := checkExternalHTTP(5 * time.Second); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
	}

	fmt.Println("Press Ctrl+C to disconnect.")
	<-ctx.Done()
	fmt.Println("\nDisconnecting...")
	return nil
}

// checkExternalHTTP fetches a URL through the system's default route to verify

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
// the VPN is passing traffic. Returns the response body snippet.
func checkExternalHTTP(timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get("https://cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	for _, line := range strings.Split(string(body), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && k == "ip" {
			fmt.Printf("  Exit IP: %s\n", v)
			return v, nil
		}
	}
	return "", fmt.Errorf("ip not found")
}

// startHealthCheck runs periodic connectivity checks. When requireCheck is
// false it's a no-op. When true, it ticks every 30 seconds and cancels the
// parent context if the external IP cannot be obtained.
func startHealthCheck(parent context.Context, socksPort int, requireCheck bool, intervalSec int, urlStr string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if !requireCheck {
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
				addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
				if _, err := check.FetchURL(addr, urlStr, 5*time.Second); err != nil {
					fmt.Printf("\nHealth check failed: %v\n", err)
					cancel()
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
