package customcertservice

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	DirectoryName       = "lazy"
	CertificateFileName = "custom-ca.pem"
	PrivateKeyFileName  = "custom-ca-key.pem"
)

const (
	privateDirMode  fs.FileMode = 0o700
	privateFileMode fs.FileMode = 0o600
)

type Paths struct {
	Dir         string
	Certificate string
	PrivateKey  string
}

type Authority struct {
	paths Paths
	cert  *x509.Certificate
	key   crypto.Signer

	mu    sync.Mutex
	cache map[string]*tls.Certificate
}

func DefaultPaths() (Paths, error) {
	dataHome, err := dataHome()
	if err != nil {
		return Paths{}, err
	}
	dir := filepath.Join(dataHome, DirectoryName)
	return Paths{
		Dir:         dir,
		Certificate: filepath.Join(dir, CertificateFileName),
		PrivateKey:  filepath.Join(dir, PrivateKeyFileName),
	}, nil
}

func LoadOrCreateDefault() (*Authority, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}
	return LoadOrCreate(paths)
}

func LoadOrCreate(paths Paths) (*Authority, error) {
	paths = completePaths(paths)
	if err := ensurePrivateDir(paths.Dir); err != nil {
		return nil, err
	}

	certPEM, certErr := os.ReadFile(paths.Certificate)
	keyPEM, keyErr := os.ReadFile(paths.PrivateKey)
	if certErr == nil && keyErr == nil {
		if err := hardenExistingPaths(paths); err != nil {
			return nil, err
		}
		return authorityFromPEM(paths, certPEM, keyPEM)
	}
	if certErr == nil || keyErr == nil {
		return nil, partialAuthorityError(paths, certErr, keyErr)
	}
	if !errors.Is(certErr, fs.ErrNotExist) || !errors.Is(keyErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("read local development CA: certificate: %w; private key: %w", certErr, keyErr)
	}

	authority, certPEM, keyPEM, err := generateAuthority(paths)
	if err != nil {
		return nil, err
	}
	if err := writePrivateFile(paths.Certificate, certPEM); err != nil {
		return nil, err
	}
	if err := writePrivateFile(paths.PrivateKey, keyPEM); err != nil {
		return nil, err
	}
	return authority, nil
}

func (a *Authority) Paths() Paths {
	if a == nil {
		return Paths{}
	}
	return a.paths
}

func (a *Authority) CertificatePEM() []byte {
	if a == nil || a.cert == nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.cert.Raw})
}

func (a *Authority) TLSConfig(defaultHost string) (*tls.Config, error) {
	if a == nil {
		return nil, fmt.Errorf("custom certificate authority is nil")
	}
	defaultHost = NormalizeHost(defaultHost)
	defaultCert, err := a.CertificateFor(defaultHost)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
		Certificates: []tls.Certificate{*defaultCert},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			host := defaultHost
			if hello != nil && strings.TrimSpace(hello.ServerName) != "" {
				host = hello.ServerName
			}
			return a.CertificateFor(host)
		},
	}, nil
}

func (a *Authority) CertificateFor(host string) (*tls.Certificate, error) {
	host = NormalizeHost(host)
	if host == "" {
		return nil, fmt.Errorf("certificate host is empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cache == nil {
		a.cache = map[string]*tls.Certificate{}
	}
	if cached := a.cache[host]; cached != nil {
		return cached, nil
	}

	cert, err := a.generateCertificateFor(host)
	if err != nil {
		return nil, err
	}
	a.cache[host] = cert
	return cert, nil
}

func NormalizeHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "localhost"
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	}
	value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	switch value {
	case "", "0.0.0.0", "::", "[::]":
		return "localhost"
	default:
		return value
	}
}

func completePaths(paths Paths) Paths {
	if paths.Dir == "" {
		paths.Dir = filepath.Dir(paths.Certificate)
	}
	if paths.Dir == "." || paths.Dir == "" {
		paths.Dir = DirectoryName
	}
	if paths.Certificate == "" {
		paths.Certificate = filepath.Join(paths.Dir, CertificateFileName)
	}
	if paths.PrivateKey == "" {
		paths.PrivateKey = filepath.Join(paths.Dir, PrivateKeyFileName)
	}
	return paths
}

func dataHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); value != "" {
		return value, nil
	}
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find user home for application data: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "windows":
		if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
			return value, nil
		}
		if value := strings.TrimSpace(os.Getenv("APPDATA")); value != "" {
			return value, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home for XDG data: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, privateDirMode); err != nil {
		return fmt.Errorf("create local certificate directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, privateDirMode); err != nil {
			return fmt.Errorf("secure local certificate directory: %w", err)
		}
	}
	return nil
}

func hardenExistingPaths(paths Paths) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chmod(paths.Dir, privateDirMode); err != nil {
		return fmt.Errorf("secure local certificate directory: %w", err)
	}
	for _, path := range []string{paths.Certificate, paths.PrivateKey} {
		if err := os.Chmod(path, privateFileMode); err != nil {
			return fmt.Errorf("secure local certificate file %s: %w", path, err)
		}
	}
	return nil
}

func authorityFromPEM(paths Paths, certPEM []byte, keyPEM []byte) (*Authority, error) {
	cert, err := parseCertificate(certPEM)
	if err != nil {
		return nil, fmt.Errorf("parse local development CA certificate: %w", err)
	}
	key, err := parsePrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse local development CA private key: %w", err)
	}
	return &Authority{paths: paths, cert: cert, key: key}, nil
}

func partialAuthorityError(paths Paths, certErr error, keyErr error) error {
	if certErr != nil && errors.Is(certErr, fs.ErrNotExist) {
		return fmt.Errorf("local development CA is incomplete: certificate file is missing at %s", paths.Certificate)
	}
	if keyErr != nil && errors.Is(keyErr, fs.ErrNotExist) {
		return fmt.Errorf("local development CA is incomplete: private key file is missing at %s", paths.PrivateKey)
	}
	return fmt.Errorf("local development CA is incomplete: certificate: %w; private key: %w", certErr, keyErr)
}

func generateAuthority(paths Paths) (*Authority, []byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate local development CA key: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Subject:               tailoredSubject(),
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create local development CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse generated local development CA certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode local development CA key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return &Authority{paths: paths, cert: cert, key: privateKey}, certPEM, keyPEM, nil
}

func tailoredSubject() pkix.Name {
	owner := machineOwner()
	commonName := "GoLazy Local Development CA"
	if owner != "" {
		commonName += " for " + owner
	}
	return pkix.Name{
		CommonName:         commonName,
		Organization:       []string{"GoLazy"},
		OrganizationalUnit: []string{"Local Development"},
	}
}

func machineOwner() string {
	user := firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))
	host, _ := os.Hostname()
	switch {
	case user != "" && host != "":
		return user + "@" + host
	case user != "":
		return user
	case host != "":
		return host
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func writePrivateFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		return fmt.Errorf("create local certificate file %s: %w", path, err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write local certificate file %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close local certificate file %s: %w", path, closeErr)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, privateFileMode); err != nil {
			return fmt.Errorf("secure local certificate file %s: %w", path, err)
		}
	}
	return nil
}

func parseCertificate(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("missing CERTIFICATE PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parsePrivateKey(data []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("missing private key PEM block")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("PKCS8 private key cannot sign certificates")
		}
		return signer, nil
	default:
		return nil, fmt.Errorf("unsupported private key PEM block %q", block.Type)
	}
}

func (a *Authority) generateCertificateFor(host string) (*tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate %s certificate key: %w", host, err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Issuer:                a.cert.Subject,
		Subject:               leafSubject(a.cert.Subject, host),
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(397 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &privateKey.PublicKey, a.key)
	if err != nil {
		return nil, fmt.Errorf("create %s certificate: %w", host, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("encode %s certificate key: %w", host, err)
	}
	certPEM := new(bytes.Buffer)
	_ = pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: a.cert.Raw})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM.Bytes(), keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load %s certificate key pair: %w", host, err)
	}
	return &cert, nil
}

func leafSubject(caSubject pkix.Name, host string) pkix.Name {
	subject := caSubject
	subject.CommonName = host
	subject.OrganizationalUnit = []string{"Local Development", host}
	return subject
}

func randomSerial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil || serial.Sign() == 0 {
		return big.NewInt(time.Now().UnixNano())
	}
	return serial
}

var _ crypto.Signer = (*ecdsa.PrivateKey)(nil)
var _ crypto.Signer = (*rsa.PrivateKey)(nil)
