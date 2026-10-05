// thinhthan-server is the single world process (ADR-0052): Edge + Sim +
// Durable + Global in one binary. This minimal main wires the IMP-006
// surface: the 12 auth/account HTTPS routes and the WSS gameplay
// listener behind the one public port, plus the admin /healthz listener.
//
// Public layout (external_integrations.md §4): SERVER_PORT is the only
// public port — it serves `/api/v1/*` directly and proxies `/ws` to the
// gameplay listener bound on loopback. TLS_TERMINATION=SERVER terminates
// TLS here with SIGHUP cert reload; =PROXY runs behind an upstream TLS
// terminator and the public listener binds loopback-only, honoring
// X-Forwarded-For's first entry from the loopback peer.
package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/auth"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	"thinhthan/internal/edge/session"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	if err := run(context.Background()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// envSpec reads a required env var.
func envSpec(name string) (string, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return "", fmt.Errorf("%s required", name)
	}
	return v, nil
}

func envInt(name string, def int) (int, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return n, nil
}

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn, err := envSpec("DATABASE_URL")
	if err != nil {
		return err
	}
	saltB64, err := envSpec("ACCOUNT_SIGNAL_SALT")
	if err != nil {
		return err
	}
	saltBytes, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil || len(saltBytes) != 32 {
		return fmt.Errorf("ACCOUNT_SIGNAL_SALT must be 32-byte base64")
	}
	var salt account.Salt
	copy(salt[:], saltBytes)

	ccuCap, err := envInt("WORLD_CCU_CAP", 0)
	if err != nil || ccuCap <= 0 {
		return fmt.Errorf("WORLD_CCU_CAP required (positive int)")
	}
	port, err := envInt("SERVER_PORT", 8080)
	if err != nil {
		return err
	}
	adminAddr, err := envSpec("ADMIN_BIND_ADDR")
	if err != nil {
		return err
	}
	tlsMode := strings.ToUpper(strings.TrimSpace(os.Getenv("TLS_TERMINATION")))
	if tlsMode != "SERVER" && tlsMode != "PROXY" {
		return fmt.Errorf("TLS_TERMINATION must be SERVER|PROXY")
	}

	pool, err := db.Pool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	store := account.NewStore(pool)

	// Startup sweep: stale session_active flags from a previous process
	// are cleared before the listener opens (session.md § Startup).
	if err := account.ClearStaleSessionActive(ctx, pool); err != nil {
		return fmt.Errorf("session_active sweep: %w", err)
	}

	// Durable queue: register the IMP-006 character.activity executor.
	q := queue.New(ccuCap, idempotency.NewStore(pool), queue.Deps{Pool: pool})
	q.RegisterExecutor(queue.ProducerActivity, store.ActivityExecutor)

	contentRevision := strings.TrimSpace(os.Getenv("CONTENT_REVISION"))
	protocolMinor, _ := envInt("PROTOCOL_MINOR", 0)
	protocolMinorMin, _ := envInt("PROTOCOL_MINOR_MIN", 0)
	protocolMajor, _ := envInt("PROTOCOL_MAJOR", 1)
	minBuild, _ := envInt("CLIENT_BUILD_MIN", 0)

	sessReg := session.New(session.Config{
		Capacity:        ccuCap,
		ProtocolMinor:   uint32(protocolMinor),
		ContentRevision: contentRevision,
	}, store, q)
	sessReg.SetRouter(router.New())

	providers := map[string]auth.ProviderClient{}
	if ids := csvEnv("GOOGLE_OIDC_CLIENT_IDS"); len(ids) > 0 {
		providers[account.ProviderGoogle] = auth.NewGoogleProvider("", ids, nil, nil)
	}
	if ids := csvEnv("APPLE_SIGNIN_CLIENT_IDS"); len(ids) > 0 {
		providers[account.ProviderApple] = auth.NewAppleProvider("", ids, nil, nil)
	}
	if appID, key := os.Getenv("STEAM_APP_ID"), os.Getenv("STEAM_PUBLISHER_KEY"); appID != "" && key != "" {
		providers[account.ProviderSteam] = auth.NewSteamProvider("", key, appID, nil)
	}

	svc := auth.New(auth.Config{
		Salt:             salt,
		Store:            store,
		Providers:        providers,
		Tickets:          sessReg,
		Revoker:          sessReg,
		WSSURL:           os.Getenv("WSS_PUBLIC_URL"),
		MinBuild:         uint32(minBuild),
		ProtocolMinorMin: uint32(protocolMinorMin),
		ContentRevision:  contentRevision,
	})
	authHandler := auth.NewHandler(svc)

	// Gameplay listener on loopback; the public server proxies /ws here.
	// TerminationProxy + TrustXFFLoopback: the peer is always our own
	// reverse proxy, and XFF's first entry carries the real client IP.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	ws, err := listener.New(listener.Config{
		Addr:             ln.Addr().String(),
		TLS:              listener.TerminationProxy,
		TrustXFFLoopback: true,
		ProtocolMajor:    uint32(protocolMajor),
		ProtocolMinor:    uint32(protocolMinor),
		MinBuild:         uint32(minBuild),
	}, listener.Deps{
		Session:    sessReg,
		Intents:    sessReg,
		Disconnect: sessReg,
	})
	if err != nil {
		return err
	}
	wsDone := make(chan error, 1)
	go func() { wsDone <- ws.ServeListener(ctx, ln) }()

	// Public mux: /api/v1/* → auth handler; /ws → loopback proxy.
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(&url.URL{Scheme: "http", Host: ln.Addr().String(), Path: "/ws"})
			// In SERVER mode we terminate TLS: the direct peer is the
			// client — any client-sent XFF is spoofable, so strip it and
			// let the proxy write RemoteAddr as the single entry.
			if tlsMode == "SERVER" {
				pr.Out.Header.Del("X-Forwarded-For")
			}
			pr.SetXForwarded()
		},
	}
	pubMux := http.NewServeMux()
	pubMux.Handle("/api/v1/", authHandler)
	pubMux.Handle("/ws", proxy)
	pubSrv := &http.Server{
		Handler:           pubMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	pubBind := fmt.Sprintf(":%d", port)
	if tlsMode == "PROXY" {
		pubBind = fmt.Sprintf("127.0.0.1:%d", port)
	}
	pubLn, err := net.Listen("tcp", pubBind)
	if err != nil {
		return err
	}

	if tlsMode == "SERVER" {
		certFile, err := envSpec("TLS_CERT_FILE")
		if err != nil {
			return err
		}
		keyFile, err := envSpec("TLS_KEY_FILE")
		if err != nil {
			return err
		}
		reload, err := newCertReloader(certFile, keyFile)
		if err != nil {
			return err
		}
		// SIGHUP reload (protocol.md § TLS).
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		go func() {
			for range sig {
				if err := reload.Reload(); err != nil {
					log.Printf("server: cert reload: %v", err)
				}
			}
		}()
		pubSrv.TLSConfig = &tls.Config{GetCertificate: reload.GetCertificate}
		pubLn = tls.NewListener(pubLn, pubSrv.TLSConfig)
	}

	// Admin listener: the startup sweep and full wiring completed above,
	// so /healthz is unconditionally ready here.
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminMux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	adminLn, err := net.Listen("tcp", adminAddr)
	if err != nil {
		return err
	}
	go func() {
		if err := adminSrv.Serve(adminLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server: admin listener: %v", err)
		}
	}()

	log.Printf("server: public=%s mode=%s admin=%s ws=%s ccu_cap=%d",
		pubBind, tlsMode, adminAddr, ln.Addr(), ccuCap)

	serveErr := make(chan error, 1)
	go func() {
		if err := pubSrv.Serve(pubLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case err := <-wsDone:
		return err
	case <-ctx.Done():
	}

	// Graceful stop: drain WS sessions, then HTTP, then the queue.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ws.Drain(shutdownCtx); err != nil {
		log.Printf("server: drain: %v", err)
	}
	_ = pubSrv.Shutdown(shutdownCtx)
	_ = adminSrv.Shutdown(shutdownCtx)
	if err := q.Shutdown(shutdownCtx); err != nil {
		log.Printf("server: queue shutdown: %v", err)
	}
	return nil
}

func csvEnv(name string) []string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// certReloader hot-swaps the TLS keypair on SIGHUP (SERVER mode).
type certReloader struct {
	certFile, keyFile string
	mu                sync.RWMutex
	current           *tls.Certificate
}

func newCertReloader(certFile, keyFile string) (*certReloader, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("TLS_CERT_FILE/TLS_KEY_FILE: %w", err)
	}
	return &certReloader{certFile: certFile, keyFile: keyFile, current: &cert}, nil
}

// Reload re-reads the keypair; failure keeps the current cert.
func (r *certReloader) Reload() error {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.current = &cert
	r.mu.Unlock()
	return nil
}

// GetCertificate implements tls.Config.GetCertificate.
func (r *certReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current, nil
}
