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
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func leafOf(t *testing.T, cfg *tls.Config) *x509.Certificate {
	t.Helper()
	if len(cfg.Certificates) != 1 {
		t.Fatalf("want 1 cert, got %d", len(cfg.Certificates))
	}
	c, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return c
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
	cfg1, ca1, err := Load(d, ips)
	if err != nil {
		t.Fatal(err)
	}
	cfg2, ca2, err := Load(d, ips)
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
	_, ca1, err := Load(d, []net.IP{net.ParseIP("192.168.1.5")})
	if err != nil {
		t.Fatal(err)
	}
	cfg, ca2, err := Load(d, []net.IP{net.ParseIP("192.168.1.5"), net.ParseIP("10.0.0.7")})
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
	if _, _, err := Load(d, []net.IP{net.ParseIP("192.168.1.5")}); err != nil {
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
	_, caPEM, err := Load(d, []net.IP{net.ParseIP("192.168.1.5")})
	if err != nil {
		t.Fatal(err)
	}
	caCert, caKey, err := readPair(filepath.Join(d, caCertFile), filepath.Join(d, caKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{CommonName: "evil"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("8.8.8.8")},
		DNSNames:     []string{"example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &k.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	evil, _ := x509.ParseCertificate(der)
	if _, err := evil.Verify(x509.VerifyOptions{Roots: pool(t, caPEM), DNSName: "example.com"}); err == nil {
		t.Fatal("CA is not name-constrained")
	}
}

func TestCAIsUsableRoot(t *testing.T) {
	_, caPEM, err := Load(t.TempDir(), []net.IP{net.ParseIP("192.168.1.5")})
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
