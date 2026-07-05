package meshjoin

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestPrivateIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", false},            // loopback
		{"127.9.9.9", false},            // loopback /8
		{"::1", false},                  // v6 loopback
		{"169.254.10.10", false},        // link-local
		{"fe80::1", false},              // v6 link-local
		{"8.8.8.8", false},              // public
		{"93.184.216.34", false},        // public
		{"2001:4860:4860::8888", false}, // public v6
		{"10.0.0.1", true},              // RFC1918
		{"172.16.5.4", true},            // RFC1918
		{"192.168.1.10", true},          // RFC1918
		{"100.64.0.1", true},            // CGNAT low edge
		{"100.127.255.254", true},       // CGNAT high edge
		{"100.63.255.255", false},       // just below CGNAT
		{"100.128.0.1", false},          // just above CGNAT
		{"fd12:3456::1", true},          // ULA
	}
	for _, c := range cases {
		if got := privateIP(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("privateIP(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
	if privateIP(nil) {
		t.Error("privateIP(nil) = true, want false")
	}
}

func TestValidateWorkerURL(t *testing.T) {
	resolveTo := func(ips ...string) func(string) ([]net.IP, error) {
		return func(string) ([]net.IP, error) {
			var out []net.IP
			for _, s := range ips {
				out = append(out, net.ParseIP(s))
			}
			return out, nil
		}
	}
	resolveErr := func(string) ([]net.IP, error) { return nil, errors.New("no such host") }

	cases := []struct {
		name    string
		raw     string
		resolve func(string) ([]net.IP, error)
		wantErr bool
		wantURL string // for success cases: the URL to register (pinned for hostnames)
	}{
		{"private ip literal", "http://10.0.0.5:4087", nil, false, "http://10.0.0.5:4087"},
		{"cgnat ip literal", "http://100.64.0.5:4087", nil, false, "http://100.64.0.5:4087"},
		{"loopback rejected", "http://127.0.0.1:4087", nil, true, ""},
		{"public ip rejected", "http://8.8.8.8:4087", nil, true, ""},
		{"link-local rejected", "http://169.254.0.9:4087", nil, true, ""},
		{"https scheme rejected", "https://10.0.0.5:4087", nil, true, ""},
		{"missing host", "http://", nil, true, ""},
		{"no scheme", "10.0.0.5:4087", nil, true, ""},
		{"hostname pinned to private ip", "http://mini.local:4087", resolveTo("10.0.0.9"), false, "http://10.0.0.9:4087"},
		{"hostname public", "http://mini.local:4087", resolveTo("93.184.216.34"), true, ""},
		{"hostname mixed private+public", "http://mini.local:4087", resolveTo("10.0.0.9", "93.184.216.34"), true, ""},
		{"hostname unresolvable", "http://mini.local:4087", resolveErr, true, ""},
		{"hostname resolves to nothing", "http://mini.local:4087", resolveTo(), true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := validateWorkerURL(c.raw, c.resolve)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateWorkerURL(%q) err = %v, wantErr %v", c.raw, err, c.wantErr)
			}
			if !c.wantErr && got != c.wantURL {
				t.Fatalf("validateWorkerURL(%q) = %q, want %q", c.raw, got, c.wantURL)
			}
		})
	}
}

func TestDetectHubURL(t *testing.T) {
	addr := func(cidr string) net.Addr {
		ip, ipn, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("bad cidr %q: %v", cidr, err)
		}
		ipn.IP = ip
		return ipn
	}
	fixed := func(as ...net.Addr) func() ([]net.Addr, error) {
		return func() ([]net.Addr, error) { return as, nil }
	}

	t.Run("first private v4 wins", func(t *testing.T) {
		got, err := detectHubURL("4087", fixed(addr("127.0.0.1/8"), addr("192.168.1.5/24"), addr("10.0.0.2/8")))
		if err != nil || got != "http://192.168.1.5:4087" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("cgnat preferred over earlier rfc1918", func(t *testing.T) {
		got, err := detectHubURL("4087", fixed(addr("192.168.1.5/24"), addr("100.64.0.7/10")))
		if err != nil || got != "http://100.64.0.7:4087" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("loopback only errors with advertise hint", func(t *testing.T) {
		_, err := detectHubURL("4087", fixed(addr("127.0.0.1/8")))
		if err == nil || !strings.Contains(err.Error(), "--advertise") {
			t.Fatalf("err = %v, want --advertise hint", err)
		}
	})
	t.Run("public only errors", func(t *testing.T) {
		if _, err := detectHubURL("4087", fixed(addr("93.184.216.34/32"))); err == nil {
			t.Fatal("want error for public-only interfaces")
		}
	})
	t.Run("ipv6 ula skipped", func(t *testing.T) {
		if _, err := detectHubURL("4087", fixed(addr("fd00::1/64"))); err == nil {
			t.Fatal("want error: v6-only has no v4 hub URL")
		}
	})
	t.Run("non-ipnet addrs skipped", func(t *testing.T) {
		got, err := detectHubURL("4087", fixed(&net.IPAddr{IP: net.ParseIP("10.0.0.9")}, addr("10.0.0.3/8")))
		if err != nil || got != "http://10.0.0.3:4087" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("addrs error propagates", func(t *testing.T) {
		boom := func() ([]net.Addr, error) { return nil, errors.New("boom") }
		if _, err := detectHubURL("4087", boom); err == nil {
			t.Fatal("want error when addrs() fails")
		}
	})
}
