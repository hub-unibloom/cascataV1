// Package server — HTTP server do Control Plane usando Chi router.
// Chi: minimalista, middleware composável, previsível, performático.
// Ref: SRS Req-2.1.2 (Chi router), SAD §A, TASK PR-5.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/cascata-platform/cascata/control-plane/internal/config"
	"github.com/cascata-platform/cascata/control-plane/internal/health"
)

// Version do Control Plane. Substituída pelo linker em builds de produção.
var Version = "0.1.0-dev"

// Server encapsula o Chi router e a configuração do HTTP server.
type Server struct {
	cfg     *config.CascataConfig
	router  chi.Router
	health  *health.Checker
}

// New cria um novo Server com o Chi router configurado.
// Registra middleware global e rotas de saúde.
func New(cfg *config.CascataConfig) *Server {
	r := chi.NewRouter()

	// Middleware stack — ordem importa
	r.Use(chimw.RequestID)    // X-Request-Id em todo request
	r.Use(chimw.RealIP)      // IP real do cliente (atrás de proxy)
	r.Use(chimw.Logger)      // Log de cada request (method, path, status, duration)
	r.Use(chimw.Recoverer)   // Recovery de panics → 500 em vez de crash

	// Timeout global: nenhum request pode levar mais que WriteTimeout
	r.Use(chimw.Timeout(cfg.HTTP.WriteTimeout))

	hc := health.NewChecker(Version)

	s := &Server{
		cfg:    cfg,
		router: r,
		health: hc,
	}

	s.routes()

	return s
}

// routes registra todas as rotas do Control Plane.
// Rotas de gestão (tenants, schemas, pools) serão adicionadas nas próximas PRs.
func (s *Server) routes() {
	// Health & readiness — acessíveis sem autenticação
	s.router.Get("/health", s.health.LiveHandler)
	s.router.Get("/ready", s.health.ReadyHandler)

	// Placeholder: rotas de API serão registradas aqui.
	// Ex: s.router.Route("/api/v1", func(r chi.Router) { ... })
}

// Run inicia o HTTP server e bloqueia até ctx ser cancelado.
// Executa graceful shutdown quando ctx.Done() sinaliza.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.HTTP.Addr,
		Handler:      s.router,
		ReadTimeout:  s.cfg.HTTP.ReadTimeout,
		WriteTimeout: s.cfg.HTTP.WriteTimeout,
	}

	// Canal para capturar erro do ListenAndServe
	errCh := make(chan error, 1)
	go func() {
		log.Printf("[cascata-cp] listening on %s (version %s)", s.cfg.HTTP.Addr, Version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server.Run: ListenAndServe: %w", err)
		}
		close(errCh)
	}()

	// Aguardar cancelamento do context OU erro fatal
	select {
	case <-ctx.Done():
		log.Printf("[cascata-cp] shutdown signal received, draining connections...")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown com timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server.Run: Shutdown: %w", err)
	}

	log.Printf("[cascata-cp] shutdown complete")
	return nil
}

// HealthChecker retorna o health checker para registro de componentes externos.
func (s *Server) HealthChecker() *health.Checker {
	return s.health
}

// SetReadyComponent é um atalho para registrar o estado de um componente.
func (s *Server) SetReadyComponent(name, status string) {
	s.health.SetComponent(name, status)
}

// WaitForReady aguarda um componente ficar pronto, com timeout.
// Usado no boot para verificar dependências antes de aceitar requests.
func WaitForReady(ctx context.Context, name string, check func() error, interval time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("WaitForReady(%s): timeout: %w", name, ctx.Err())
		default:
			if err := check(); err == nil {
				return nil
			}
			time.Sleep(interval)
		}
	}
}
