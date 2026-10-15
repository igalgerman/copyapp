package tlsutil

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixed(ips ...net.IP) func() []net.IP { return func() []net.IP { return ips } }

// leafOf returns the certificate a handshake would be served.
func leafOf(t *testing.T, cfg *tls.Config) *x509.Certificate {
	t.Helper()
	tc, err := cfg.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil || tc == nil || len(tc.Certificate) != 1 {
		t.Fatalf("GetCertificate: %v %v", tc, err)
	}
	c, err := x509.ParseCertificate(tc.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// handshake runs a real TLS handshake verifying the server as host.
func handshake(cfg *tls.Config, caPEM []byte, host string) error {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go tls.Server(a, cfg).Handshake()
	return tls.Client(b, &tls.Config{RootCAs: roots, ServerName: host}).Handshake()
}

func TestLeafFollowsIPChangeAtRuntime(t *testing.T) {
	defer func(d time.Duration) { recheckInterval = d }(recheckInterval)
	recheckInterval = 0
	d := t.TempDir()
	ips := []net.IP{net.ParseIP("192.168.1.5")}
	cfg, caPEM, err := Load(d, func() []net.IP { return ips }, fakeProtector{})
	if err != nil {
		t.Fatal(err)
	}
	if err := handshake(cfg, caPEM, "192.168.1.5"); err != nil {
		t.Fatalf("initial IP: %v", err)
	}
	first := leafOf(t, cfg)

	ips = []net.IP{net.ParseIP("10.0.0.7")} // e.g. DHCP gave a new address
	if err := handshake(cfg, caPEM, "10.0.0.7"); err != nil {
		t.Fatalf("new IP must be served without restart: %v", err)
	}
	second := leafOf(t, cfg)
	if bytes.Equal(first.Raw, second.Raw) {
		t.Fatal("leaf not regenerated")
	}
	if !bytes.Equal(leafOf(t, cfg).Raw, second.Raw) {
		t.Fatal("leaf regenerated without reason")
	}
	// Persisted: a restart serves the new leaf under the same CA.
	cfg2, caPEM2, err := Load(d, func() []net.IP { return ips }, fakeProtector{})
	if err != nil || !bytes.Equal(caPEM, caPEM2) || !bytes.Equal(leafOf(t, cfg2).Raw, second.Raw) {
		t.Fatalf("restart after regeneration: %v", err)
	}
}

func TestLeafRecheckIsRateLimited(t *testing.T) {
	calls := 0
	cfg, _, err := Load(t.TempDir(), func() []net.IP { calls++; return []net.IP{net.ParseIP("192.168.1.5")} }, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		leafOf(t, cfg)
	}
	if calls > 2 { // Load + at most one recheck within the interval
		t.Fatalf("LAN IPs read %d times", calls)
	}
}

func pool(t *testing.T, caPEM []byte) *x509.CertPool {
	t.Helper()
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(caPEM) {
		t.Fatal("bad CA PEM")
	}
	return p
}

func TestLoadStableCAAndLeafVerifies(t *testing.T) {
	d := t.TempDir()
	ips := []net.IP{net.ParseIP("192.168.1.5")}
	cfg1, ca1, err := Load(d, fixed(ips...), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg2, ca2, err := Load(d, fixed(ips...), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca1, ca2) {
		t.Fatal("CA changed between loads")
	}
	l1, l2 := leafOf(t, cfg1), leafOf(t, cfg2)
	if !bytes.Equal(l1.Raw, l2.Raw) {
		t.Fatal("leaf regenerated without reason")
	}
	for _, host := range []string{"192.168.1.5", "copyapp.local"} {
		if _, err := l1.Verify(x509.VerifyOptions{Roots: pool(t, ca1), DNSName: host}); err != nil {
			t.Fatalf("leaf does not verify for %s: %v", host, err)
		}
	}
	if cfg1.MinVersion < tls.VersionTLS12 {
		t.Fatal("min TLS version too low")
	}
}

func TestNewIPRegeneratesLeafKeepsCA(t *testing.T) {
	d := t.TempDir()
	_, ca1, err := Load(d, fixed(net.ParseIP("192.168.1.5")), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, ca2, err := Load(d, fixed(net.ParseIP("192.168.1.5"), net.ParseIP("10.0.0.7")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca1, ca2) {
		t.Fatal("CA must be kept when IPs change")
	}
	if _, err := leafOf(t, cfg).Verify(x509.VerifyOptions{Roots: pool(t, ca2), DNSName: "10.0.0.7"}); err != nil {
		t.Fatalf("new IP not in leaf: %v", err)
	}
}

func TestKeyFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows; %APPDATA% ACLs apply")
	}
	d := t.TempDir()
	if _, _, err := Load(d, fixed(net.ParseIP("192.168.1.5")), nil); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{caKeyFile, leafKeyFile} {
		st, err := os.Stat(filepath.Join(d, f))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s has mode %v", f, st.Mode().Perm())
		}
	}
}

// The CA is name-constrained: a leaf it signs for a public IP must not verify.
func TestCANameConstrained(t *testing.T) {
	d := t.TempDir()
	_, caPEM, err := Load(d, fixed(net.ParseIP("192.168.1.5")), nil)
	if err != nil {
		t.Fatal(err)
	}
	caCert, caKey, err := readPair(filepath.Join(d, caCertFile), filepath.Join(d, caKeyFile), nil)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(ips []net.IP, names []string) *x509.Certificate {
		k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(99),
			Subject:      pkix.Name{CommonName: "evil"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			IPAddresses:  ips,
			DNSNames:     names,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &k.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := x509.ParseCertificate(der)
		return c
	}
	for _, tc := range []struct {
		host  string
		ips   []net.IP
		names []string
	}{
		{"8.8.8.8", []net.IP{net.ParseIP("8.8.8.8")}, nil},                           // public IPv4 alone
		{"2001:4860:4860::8888", []net.IP{net.ParseIP("2001:4860:4860::8888")}, nil}, // public IPv6
		{"example.com", nil, []string{"example.com"}},                                // public name alone
		{"copyapp.local.evil.com", nil, []string{"copyapp.local.evil.com"}},          // suffix trick
		{"example.com", []net.IP{net.ParseIP("8.8.8.8")}, []string{"example.com"}},
	} {
		c := issue(tc.ips, tc.names)
		if _, err := c.Verify(x509.VerifyOptions{Roots: pool(t, caPEM), DNSName: tc.host}); err == nil {
			t.Errorf("leaf for %s verified; CA is not name-constrained", tc.host)
		}
	}
	// Sanity: the same path works for a permitted private IP.
	ok := issue([]net.IP{net.ParseIP("10.1.2.3")}, nil)
	if _, err := ok.Verify(x509.VerifyOptions{Roots: pool(t, caPEM), DNSName: "10.1.2.3"}); err != nil {
		t.Fatalf("private IP should verify: %v", err)
	}
}

// fakeProtector stands in for DPAPI so the tests stay hermetic.
type fakeProtector struct{}

func (fakeProtector) Protect(b []byte) ([]byte, error) {
	out := []byte("fake:")
	for _, x := range b {
		out = append(out, x^0x5a)
	}
	return out, nil
}

func (fakeProtector) Unprotect(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, []byte("fake:")) {
		return nil, errors.New("not protected by fake")
	}
	out := make([]byte, 0, len(b)-5)
	for _, x := range b[5:] {
		out = append(out, x^0x5a)
	}
	return out, nil
}

func assertProtected(t *testing.T, d string) {
	t.Helper()
	for _, f := range []string{caKeyFile, leafKeyFile} {
		b, err := os.ReadFile(filepath.Join(d, f))
		if err != nil {
			t.Fatal(err)
		}
		blk, _ := pem.Decode(b)
		if blk == nil || blk.Type != protectedKeyType || bytes.Contains(b, []byte("BEGIN EC PRIVATE KEY")) {
			t.Fatalf("%s is not protected at rest:\n%s", f, b)
		}
	}
}

func TestKeysProtectedAtRest(t *testing.T) {
	d := t.TempDir()
	ips := []net.IP{net.ParseIP("192.168.1.5")}
	cfg1, ca1, err := Load(d, fixed(ips...), fakeProtector{})
	if err != nil {
		t.Fatal(err)
	}
	assertProtected(t, d)
	cfg2, ca2, err := Load(d, fixed(ips...), fakeProtector{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca1, ca2) || !bytes.Equal(leafOf(t, cfg1).Raw, leafOf(t, cfg2).Raw) {
		t.Fatal("protected keys did not load back")
	}
}

func TestPlaintextKeysMigrated(t *testing.T) {
	d := t.TempDir()
	ips := []net.IP{net.ParseIP("192.168.1.5")}
	cfg1, ca1, err := Load(d, fixed(ips...), nil) // as written by older versions
	if err != nil {
		t.Fatal(err)
	}
	cfg2, ca2, err := Load(d, fixed(ips...), fakeProtector{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca1, ca2) || !bytes.Equal(leafOf(t, cfg1).Raw, leafOf(t, cfg2).Raw) {
		t.Fatal("migration must keep the CA and leaf")
	}
	assertProtected(t, d)
}

func TestUnreadableCAKeyIsNotReplaced(t *testing.T) {
	d := t.TempDir()
	ips := []net.IP{net.ParseIP("192.168.1.5")}
	if _, _, err := Load(d, fixed(ips...), fakeProtector{}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(d, caCertFile))
	if _, _, err := Load(d, fixed(ips...), nil); err == nil {
		t.Fatal("a protected CA key without its protector must fail, not be regenerated")
	}
	after, _ := os.ReadFile(filepath.Join(d, caCertFile))
	if !bytes.Equal(before, after) {
		t.Fatal("CA was replaced")
	}
}

func TestCAIsUsableRoot(t *testing.T) {
	_, caPEM, err := Load(t.TempDir(), fixed(net.ParseIP("192.168.1.5")), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := pem.Decode(caPEM)
	c, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsCA || c.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Fatal("CA cert lacks CA/CertSign")
	}
	if c.NotAfter.Before(time.Now().AddDate(4, 11, 0)) {
		t.Fatal("CA validity should be ~5y")
	}
	if len(Fingerprint(caPEM)) != 64+31 {
		t.Fatalf("fingerprint format: %q", Fingerprint(caPEM))
	}
}

func TestShortFingerprint(t *testing.T) {
	_, caPEM, err := Load(t.TempDir(), fixed(net.ParseIP("192.168.1.5")), nil)
	if err != nil {
		t.Fatal(err)
	}
	full := strings.ReplaceAll(Fingerprint(caPEM), ":", "")
	want := full[0:4] + " " + full[4:8] + " " + full[8:12] + " " + full[12:16]
	if got := ShortFingerprint(caPEM); got != want {
		t.Fatalf("short %q, want %q", got, want)
	}
	if ShortFingerprint([]byte("junk")) != "" {
		t.Fatal("junk must give empty")
	}
}
