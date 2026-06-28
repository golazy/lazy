package customcertservice

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultPathsUseXDGDataHome(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}

	wantDir := filepath.Join(dataHome, "lazy")
	if paths.Dir != wantDir {
		t.Fatalf("Dir = %q, want %q", paths.Dir, wantDir)
	}
	if paths.Certificate != filepath.Join(wantDir, CertificateFileName) {
		t.Fatalf("Certificate = %q", paths.Certificate)
	}
	if paths.PrivateKey != filepath.Join(wantDir, PrivateKeyFileName) {
		t.Fatalf("PrivateKey = %q", paths.PrivateKey)
	}
}

func TestLoadOrCreateWritesPrivateFilesAndReusesAuthority(t *testing.T) {
	paths := testPaths(t)

	authority, err := LoadOrCreate(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(authority.CertificatePEM()) == 0 {
		t.Fatal("CertificatePEM is empty")
	}
	assertMode(t, paths.Dir, 0o700)
	assertMode(t, paths.Certificate, 0o600)
	assertMode(t, paths.PrivateKey, 0o600)

	reloaded, err := LoadOrCreate(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !authority.cert.Equal(reloaded.cert) {
		t.Fatal("LoadOrCreate regenerated the CA instead of reusing it")
	}
}

func TestCertificateForDNSName(t *testing.T) {
	authority, err := LoadOrCreate(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}

	cert, err := authority.CertificateFor("dev.local:3000")
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) != 2 {
		t.Fatalf("certificate chain length = %d, want 2", len(cert.Certificate))
	}
	leaf := parseLeaf(t, cert)
	if leaf.Subject.CommonName != "dev.local" {
		t.Fatalf("CommonName = %q, want dev.local", leaf.Subject.CommonName)
	}
	if got := strings.Join(leaf.DNSNames, ","); got != "dev.local" {
		t.Fatalf("DNSNames = %q, want dev.local", got)
	}

	again, err := authority.CertificateFor("dev.local")
	if err != nil {
		t.Fatal(err)
	}
	if again != cert {
		t.Fatal("CertificateFor did not cache the generated domain certificate")
	}
}

func TestCertificateForIPAddress(t *testing.T) {
	authority, err := LoadOrCreate(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}

	cert, err := authority.CertificateFor("127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}
	leaf := parseLeaf(t, cert)
	if len(leaf.IPAddresses) != 1 || !leaf.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("IPAddresses = %#v, want 127.0.0.1", leaf.IPAddresses)
	}
}

func TestTLSConfigUsesDefaultCertificateWhenSNIIsEmpty(t *testing.T) {
	authority, err := LoadOrCreate(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}
	config, err := authority.TLSConfig("127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := config.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	leaf := parseLeaf(t, cert)
	if len(leaf.IPAddresses) != 1 || !leaf.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("IPAddresses = %#v, want 127.0.0.1", leaf.IPAddresses)
	}
	if len(config.NextProtos) == 0 || config.NextProtos[0] != "h2" {
		t.Fatalf("NextProtos = %#v, want h2 first", config.NextProtos)
	}
}

func TestPartialAuthorityFails(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Certificate, []byte("not a cert"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadOrCreate(paths)
	if err == nil || !strings.Contains(err.Error(), "private key file is missing") {
		t.Fatalf("error = %v, want missing private key", err)
	}
}

func testPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		Dir:         dir,
		Certificate: filepath.Join(dir, CertificateFileName),
		PrivateKey:  filepath.Join(dir, PrivateKeyFileName),
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}

func parseLeaf(t *testing.T, cert *tls.Certificate) *x509.Certificate {
	t.Helper()
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf
}
