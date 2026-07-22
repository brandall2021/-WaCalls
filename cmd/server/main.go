package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wacalls/internal/sip"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	databaseURL := flag.String("database-url", "", "PostgreSQL connection URL (required)")
	staticDir := flag.String("static", "client/dist", "static client directory (optional)")
	debug := flag.Bool("debug", false, "verbose logging")
	maxCalls := flag.Int("max-calls-per-session", 8, "max concurrent calls per session (0 = unlimited)")

	// SIP bridge flags
	sipEnabled := flag.Bool("sip", false, "enable SIP bridge to Asterisk")
	sipAddr := flag.String("sip-addr", "", "Asterisk SIP address (host:port)")
	sipExt := flag.String("sip-ext", "", "SIP extension number")
	sipPass := flag.String("sip-pass", "", "SIP extension password")
	sipDomain := flag.String("sip-domain", "", "SIP domain (default: sip-addr)")

	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
	}
	if *databaseURL == "" {
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		log.Error("DATABASE_URL is required (use -database-url flag or set DATABASE_URL env)")
		os.Exit(1)
	}

	// SIP env fallbacks
	if !*sipEnabled {
		*sipEnabled = os.Getenv("SIP_ENABLED") == "true"
	}
	if *sipAddr == "" {
		*sipAddr = os.Getenv("SIP_ADDR")
	}
	if *sipExt == "" {
		*sipExt = os.Getenv("SIP_EXT")
	}
	if *sipPass == "" {
		*sipPass = os.Getenv("SIP_PASS")
	}
	if *sipDomain == "" {
		*sipDomain = os.Getenv("SIP_DOMAIN")
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var sipConfig *sip.Config
	if *sipEnabled && *sipAddr != "" && *sipExt != "" {
		sipConfig = &sip.Config{
			AsteriskAddr: *sipAddr,
			Extension:    *sipExt,
			Password:     *sipPass,
			Domain:       *sipDomain,
			ListenAddr:   ":0",
		}
		log.Info("SIP bridge enabled", "asterisk", *sipAddr, "ext", *sipExt)
	}

	srv, err := newServer(ctx, *databaseURL, *staticDir, *maxCalls, sipConfig, log)
	if err != nil {
		log.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer srv.sessions.disconnectAll()

	if srv.sipUA != nil {
		defer srv.sipUA.Stop()
	}

	if err := srv.auth.Seed(ctx); err != nil {
		log.Error("seed failed", "err", err)
	}
	log.Info("seed complete")

	if err := srv.sessions.Restore(ctx); err != nil {
		log.Error("session restore failed", "err", err)
		os.Exit(1)
	}

	httpSrv := &http.Server{Addr: *addr, Handler: srv.routes()}
	go func() {
		log.Info("HTTP server listening", "addr", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
