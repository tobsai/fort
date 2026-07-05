package meshjoin

import (
	"fmt"
	"net"
	"net/url"
)

// privateIP reports whether ip is on a network we trust with the cleartext
// mesh token: RFC1918, CGNAT 100.64/10 (Tailscale), or IPv6 ULA. Loopback and
// anything public are rejected (spec 024 wire validation).
func privateIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.IsPrivate() { // RFC1918 + ULA fc00::/7
		return true
	}
	if cgnat := (&net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}); cgnat.Contains(ip) {
		return true
	}
	return false
}

// validateWorkerURL enforces spec 024 on a derived or advertised worker URL:
// http scheme, parsable host, and every address it names must be private. It
// returns the URL to actually register: for an IP literal, raw unchanged; for
// a hostname, the URL PINNED to a validated literal IP. Pinning closes a
// DNS-rebinding hole — without it the transport would re-resolve the hostname
// at dispatch time, when an attacker's DNS could answer with a public address
// even though it answered private during validation.
func validateWorkerURL(raw string, resolve func(string) ([]net.IP, error)) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" {
		return "", fmt.Errorf("meshjoin: url %q must be http://host:port", raw)
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if !privateIP(ip) {
			return "", fmt.Errorf("meshjoin: %s is not a private/tailnet address", host)
		}
		return raw, nil
	}
	ips, err := resolve(host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("meshjoin: cannot resolve %q", host)
	}
	for _, ip := range ips {
		if !privateIP(ip) {
			return "", fmt.Errorf("meshjoin: %s resolves to non-private %s", host, ip)
		}
	}
	// Pin to the first validated IP so the transport connects to exactly the
	// address that passed. Preserve the port (u.Port() is "" if none).
	pinned := *u
	if port := u.Port(); port != "" {
		pinned.Host = net.JoinHostPort(ips[0].String(), port)
	} else {
		pinned.Host = ips[0].String()
	}
	return pinned.String(), nil
}

// detectHubURL picks the hub's advertised URL: the first non-loopback
// interface address, preferring the CGNAT/Tailscale 100.64/10 range.
func detectHubURL(port string, addrs func() ([]net.Addr, error)) (string, error) {
	list, err := addrs()
	if err != nil {
		return "", err
	}
	var fallback string
	for _, a := range list {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.To4() == nil || !privateIP(ipn.IP) {
			continue
		}
		u := fmt.Sprintf("http://%s:%s", ipn.IP, port)
		if cgnat := (&net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}); cgnat.Contains(ipn.IP) {
			return u, nil // tailscale address wins immediately
		}
		if fallback == "" {
			fallback = u
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("meshjoin: no non-loopback private interface found; pass --advertise")
	}
	return fallback, nil
}
