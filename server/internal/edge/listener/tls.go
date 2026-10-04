package listener

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

// TerminationMode selects the TLS model of external_integrations.md §4:
// TLS_TERMINATION=SERVER terminates TLS in this process with
// TLS_CERT_FILE/TLS_KEY_FILE (reloaded on SIGHUP); TLS_TERMINATION=PROXY
// binds the listener to a loopback address and honors X-Forwarded-For only
// from a loopback peer.
type TerminationMode int

const (
	TerminationServer TerminationMode = iota
	TerminationProxy
)

// certReloader reloads TLS_CERT_FILE/TLS_KEY_FILE on SIGHUP so operator
// rotation needs no restart (external_integrations.md §4).
type certReloader struct {
	certFile string
	keyFile  string
	cur      atomic.Pointer[tls.Certificate]
}

func newCertReloader(certFile, keyFile string) (*certReloader, error) {
	r := &certReloader{certFile: certFile, keyFile: keyFile}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *certReloader) reload() error {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return fmt.Errorf("edge/listener: load TLS keypair: %w", err)
	}
	r.cur.Store(&cert)
	return nil
}

func (r *certReloader) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return r.cur.Load(), nil
}

// tlsConfig builds the SERVER-mode tls.Config whose certificate is
// resolved per handshake from the SIGHUP-reloaded keypair.
func tlsConfig(r *certReloader) *tls.Config {
	return &tls.Config{
		GetCertificate: r.getCertificate,
		MinVersion:     tls.VersionTLS12,
	}
}

// watchSIGHUP reloads the keypair whenever the process gets SIGHUP. Returns
// a stop function; called once per listener in SERVER mode.
func (r *certReloader) watchSIGHUP(logf func(string, ...any)) func() {
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ch:
				if err := r.reload(); err != nil && logf != nil {
					logf("listener: SIGHUP cert reload failed: %v", err)
				}
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

// clientIP resolves the effective client address: in PROXY mode the
// X-Forwarded-For header is honored only when the TCP peer is loopback
// (external_integrations.md §4); otherwise the header is ignored.
func clientIP(r *http.Request, trustXFFLoopback bool) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	peer := net.ParseIP(host)
	if err != nil || peer == nil {
		peer = net.ParseIP(r.RemoteAddr)
	}
	if trustXFFLoopback && peer != nil && peer.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// First entry is the original client.
			for i := 0; i < len(xff); i++ {
				if xff[i] == ',' {
					xff = xff[:i]
					break
				}
			}
			if ip := net.ParseIP(trimSpace(xff)); ip != nil {
				return ip
			}
		}
	}
	return peer
}

func trimSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}
