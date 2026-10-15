// Package tlsutil creates and persists a small private CA and a leaf
// certificate for the LAN HTTPS server.
//
// The CA is installed once on the phone. It is name-constrained to private
// IP ranges and the copyapp.local/localhost names, so even if its key leaked
// it could not be used to impersonate public sites. The leaf is regenerated
// whenever the machine's LAN IPs change; the CA is kept so the phone does not
// have to re-install it.
package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	caCertFile   = "ca.crt"
	caKeyFile    = "ca.key"
	leafCertFile = "leaf.crt"
	leafKeyFile  = "leaf.key"

	// DNSName is the host name included in the leaf certificate.
	DNSName = "copyapp.local"

	caValidity   = 5 * 365 * 24 * time.Hour
	leafValidity = 365 * 24 * time.Hour
	renewBefore  = 30 * 24 * time.Hour
)

// permittedNets are the IP ranges the CA may issue for.
var permittedNets = mustCIDRs(
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", // RFC 1918
	"127.0.0.0/8", "169.254.0.0/16", // loopback, link-local
	"::1/128", "fc00::/7", "fe80::/10",
)

var permittedDNS = []string{DNSName, "localhost"}

func mustCIDRs(ss ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(ss))
	for _, s := range ss {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}

// KeyProtector encrypts private keys at rest (Windows DPAPI in the app; a fake
// in tests). A nil KeyProtector stores keys as plain PEM.
type KeyProtector interface {
	Protect(plain []byte) ([]byte, error)
	Unprotect(enc []byte) ([]byte, error)
}

// protectedKeyType is the PEM block type of a key encrypted by a KeyProtector.
const protectedKeyType = "COPYAPP PROTECTED EC PRIVATE KEY"

// recheckInterval is how often a handshake may re-read the LAN IPs.
var recheckInterval = 15 * time.Second

// Load returns a TLS config serving a leaf certificate for the current LAN
// IPs (plus 127.0.0.1, localhost and copyapp.local), and the CA certificate in
// PEM form for installing on the phone. Keys and certificates are stored in
// dir with mode 0600, keys encrypted with kp; they are created on first use.
// Plain keys from older versions are re-encrypted with kp when read.
//
// ips is re-read at most every 15 s during handshakes; when an address is
// missing from the leaf (DHCP renewal, another Wi-Fi) a new leaf is issued by
// the same CA, so the phone never sees a certificate error. Note that the
// phone's session cookie and installed PWA are tied to the origin
// (https://IP:port), so a new IP still means opening the new address.
func Load(dir string, ips func() []net.IP, kp KeyProtector) (*tls.Config, []byte, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}
	caCert, caKey, err := loadOrCreateCA(dir, kp)
	if err != nil {
		return nil, nil, err
	}
	ls := &leafSource{dir: dir, ca: caCert, caKey: caKey, kp: kp, ips: ips}
	want := ls.want()
	leaf, err := loadLeaf(dir, caCert, want, kp)
	if err != nil {
		leaf, err = createLeaf(dir, caCert, caKey, want, kp)
		if err != nil {
			return nil, nil, err
		}
	}
	ls.cert, ls.checked = &leaf, time.Now()
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	cfg := &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: ls.get,
	}
	return cfg, caPEM, nil
}

// leafSource serves the leaf and replaces it when the LAN IPs change.
type leafSource struct {
	dir   string
	ca    *x509.Certificate
	caKey *ecdsa.PrivateKey
	kp    KeyProtector
	ips   func() []net.IP

	mu      sync.Mutex
	cert    *tls.Certificate
	checked time.Time
}

func (l *leafSource) want() []net.IP {
	return append([]net.IP{net.IPv4(127, 0, 0, 1)}, l.ips()...)
}

func (l *leafSource) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.checked) < recheckInterval {
		return l.cert, nil
	}
	l.checked = time.Now()
	want := l.want()
	if usable(l.cert.Leaf, want) == nil {
		return l.cert, nil
	}
	leaf, err := createLeaf(l.dir, l.ca, l.caKey, want, l.kp)
	if err != nil {
		return l.cert, nil // keep serving the old leaf; retry after the interval
	}
	l.cert = &leaf
	return l.cert, nil
}

// Fingerprint returns the SHA-256 fingerprint of the first certificate in
// certPEM as colon-separated upper-case hex (as shown by Android settings).
func Fingerprint(certPEM []byte) string {
	b, _ := pem.Decode(certPEM)
	if b == nil {
		return ""
	}
	sum := sha256.Sum256(b.Bytes)
	parts := make([]string, len(sum))
	for i, x := range sum {
		parts[i] = fmt.Sprintf("%02X", x)
	}
	return strings.Join(parts, ":")
}

func loadOrCreateCA(dir string, kp KeyProtector) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cp, kpath := filepath.Join(dir, caCertFile), filepath.Join(dir, caKeyFile)
	_, e1 := os.Stat(cp)
	_, e2 := os.Stat(kpath)
	if e1 == nil && e2 == nil {
		// Do not silently replace a damaged CA: the phone trusts it.
		return readPair(cp, kpath, kp)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:                serial(),
		Subject:                     pkix.Name{CommonName: "CopyApp Local CA", Organization: []string{"CopyApp"}},
		NotBefore:                   now.Add(-time.Hour),
		NotAfter:                    now.Add(caValidity),
		IsCA:                        true,
		BasicConstraintsValid:       true,
		MaxPathLenZero:              true,
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         permittedDNS,
		PermittedIPRanges:           permittedNets,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	if err := writePair(cp, kpath, der, key, kp); err != nil {
		return nil, nil, err
	}
	// A new CA invalidates any old leaf.
	os.Remove(filepath.Join(dir, leafCertFile))
	os.Remove(filepath.Join(dir, leafKeyFile))
	c, err := x509.ParseCertificate(der)
	return c, key, err
}

// loadLeaf returns the stored leaf if it is signed by ca, covers every IP in
// want, and is not close to expiry.
func loadLeaf(dir string, ca *x509.Certificate, want []net.IP, kp KeyProtector) (tls.Certificate, error) {
	cp, kpath := filepath.Join(dir, leafCertFile), filepath.Join(dir, leafKeyFile)
	c, key, err := readPair(cp, kpath, kp)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := c.CheckSignatureFrom(ca); err != nil {
		return tls.Certificate{}, err
	}
	if err := usable(c, want); err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{c.Raw}, PrivateKey: key, Leaf: c}, nil
}

// usable checks that the leaf covers every IP in want and is not near expiry.
func usable(c *x509.Certificate, want []net.IP) error {
	if time.Until(c.NotAfter) < renewBefore {
		return errors.New("leaf expiring")
	}
	for _, ip := range want {
		found := false
		for _, have := range c.IPAddresses {
			if have.Equal(ip) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("leaf missing IP")
		}
	}
	return nil
}

func createLeaf(dir string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, ips []net.IP, kp KeyProtector) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	var sans []net.IP
	for _, ip := range ips {
		dup := false
		for _, s := range sans {
			dup = dup || s.Equal(ip)
		}
		if !dup && ip != nil {
			sans = append(sans, ip)
		}
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: DNSName},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(leafValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     permittedDNS,
		IPAddresses:  sans,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := writePair(filepath.Join(dir, leafCertFile), filepath.Join(dir, leafKeyFile), der, key, kp); err != nil {
		return tls.Certificate{}, err
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: c}, nil
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		panic(err)
	}
	return n.Add(n, big.NewInt(1))
}

// readPair loads a certificate and its key. A plain key is re-written
// encrypted when kp is set (migration from older versions; best effort).
func readPair(certPath, keyPath string, kp KeyProtector) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cb, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	kb, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	cblk, _ := pem.Decode(cb)
	kblk, _ := pem.Decode(kb)
	if cblk == nil || kblk == nil {
		return nil, nil, fmt.Errorf("tlsutil: bad PEM in %s or %s", certPath, keyPath)
	}
	c, err := x509.ParseCertificate(cblk.Bytes)
	if err != nil {
		return nil, nil, err
	}
	kder := kblk.Bytes
	switch kblk.Type {
	case protectedKeyType:
		if kp == nil {
			return nil, nil, fmt.Errorf("tlsutil: %s is encrypted and no key protector is set", keyPath)
		}
		if kder, err = kp.Unprotect(kder); err != nil {
			return nil, nil, fmt.Errorf("tlsutil: decrypt %s: %w", keyPath, err)
		}
	case "EC PRIVATE KEY":
	default:
		return nil, nil, fmt.Errorf("tlsutil: unexpected PEM type in %s", keyPath)
	}
	k, err := x509.ParseECPrivateKey(kder)
	if err != nil {
		return nil, nil, err
	}
	pub, ok := c.PublicKey.(*ecdsa.PublicKey)
	if !ok || !pub.Equal(&k.PublicKey) {
		return nil, nil, fmt.Errorf("tlsutil: key does not match %s", certPath)
	}
	if kblk.Type != protectedKeyType && kp != nil {
		_ = writeKey(keyPath, k, kp)
	}
	return c, k, nil
}

func writeKey(keyPath string, key *ecdsa.PrivateKey, kp KeyProtector) error {
	kder, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	blk := &pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}
	if kp != nil {
		enc, err := kp.Protect(kder)
		if err != nil {
			return err
		}
		blk = &pem.Block{Type: protectedKeyType, Bytes: enc}
	}
	return writeFile(keyPath, pem.EncodeToMemory(blk))
}

func writePair(certPath, keyPath string, der []byte, key *ecdsa.PrivateKey, kp KeyProtector) error {
	if err := writeKey(keyPath, key, kp); err != nil {
		return err
	}
	return writeFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// writeFile writes atomically. os.CreateTemp creates the file with mode 0600
// (on Windows the per-user %APPDATA% ACL protects it instead).
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		os.Remove(name)
	}
	return werr
}
