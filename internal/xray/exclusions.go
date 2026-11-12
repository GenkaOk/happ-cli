package xray

import (
	"fmt"
	"net"
	"strings"
)

// Exclusion is one normalized exclude entry: either a domain pattern for
// xray's domain matcher or an IPv4 CIDR for ip rules.
type Exclusion struct {
	Domain string
	CIDR   string
}

// domainPrefixes are the xray domain matcher prefixes that pass through
// unchanged. A bare domain gets "domain:" prepended (matches subdomains too).
var domainPrefixes = []string{"domain:", "full:", "keyword:", "regexp:"}

// ParseExclusions normalizes raw lines/values: blank lines and # comments are
// skipped; "a.b/24" or an IP becomes a CIDR (bare IP -> /32, IPv6 rejected);
// bare domains become "domain:<name>" (matches subdomains); "domain:", "full:",
// "keyword:" and "regexp:" prefixes pass through; anything else is reported as
// an error and skipped.
//
// Errors are returned separately from the accepted entries so callers can warn
// and keep going: one typo must not prevent the tunnel from starting.
func ParseExclusions(raw []string) (exclusions []Exclusion, errs []error) {
	for _, line := range raw {
		entry := normalizeExcludeLine(line)
		if entry == "" {
			continue
		}
		e, err := parseExclusion(entry)
		if err != nil {
			errs = append(errs, fmt.Errorf("%q: %w", entry, err))
			continue
		}
		exclusions = append(exclusions, e)
	}
	return exclusions, errs
}

// normalizeExcludeLine trims whitespace and CR (files may come from Windows),
// drops whole-line comments, and strips a trailing " # comment".
func normalizeExcludeLine(line string) string {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	for i := 1; i < len(line); i++ {
		if line[i] == '#' && (line[i-1] == ' ' || line[i-1] == '\t') {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}

// parseExclusion classifies one normalized entry.
func parseExclusion(entry string) (Exclusion, error) {
	for _, prefix := range domainPrefixes {
		if strings.HasPrefix(entry, prefix) {
			pattern := strings.TrimSpace(strings.TrimPrefix(entry, prefix))
			if pattern == "" {
				return Exclusion{}, fmt.Errorf("empty %s pattern", strings.TrimSuffix(prefix, ":"))
			}
			return Exclusion{Domain: prefix + pattern}, nil
		}
	}

	// A space inside a bare entry is never valid (it is not a domain, an IP or
	// a CIDR), and silently accepting it would produce a matcher that never
	// fires.
	if strings.ContainsAny(entry, " \t") {
		return Exclusion{}, fmt.Errorf("unexpected whitespace")
	}

	if strings.Contains(entry, "/") {
		_, ipnet, err := net.ParseCIDR(entry)
		if err != nil {
			if ip := net.ParseIP(entry); ip != nil && ip.To4() == nil {
				return Exclusion{}, fmt.Errorf("IPv6 is not supported")
			}
			return Exclusion{}, fmt.Errorf("invalid CIDR: %w", err)
		}
		if ipnet.IP.To4() == nil {
			return Exclusion{}, fmt.Errorf("IPv6 is not supported")
		}
		// Normalize to the network address (10.1.2.3/8 -> 10.0.0.0/8) so the
		// generated ip rule and xray's ip matcher agree.
		return Exclusion{CIDR: ipnet.String()}, nil
	}

	if ip := net.ParseIP(entry); ip != nil {
		if ip.To4() == nil {
			return Exclusion{}, fmt.Errorf("IPv6 is not supported")
		}
		return Exclusion{CIDR: ip.String() + "/32"}, nil
	}

	return Exclusion{Domain: "domain:" + entry}, nil
}

// ExcludedCIDRs returns the IPv4 CIDRs of exclusions, preserving input order.
func ExcludedCIDRs(exclusions []Exclusion) []string {
	var out []string
	for _, e := range exclusions {
		if e.CIDR != "" {
			out = append(out, e.CIDR)
		}
	}
	return out
}

// ExcludedDomains returns the domain patterns of exclusions, preserving input
// order.
func ExcludedDomains(exclusions []Exclusion) []string {
	var out []string
	for _, e := range exclusions {
		if e.Domain != "" {
			out = append(out, e.Domain)
		}
	}
	return out
}
