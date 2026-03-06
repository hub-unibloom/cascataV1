// Package main — Entrypoint do Control Plane do Cascata.
// Carrega configuração, inicia Chi router, escuta requests.
// Graceful shutdown em SIGINT/SIGTERM.
// Toda goroutine com context cancelável (Regra 5.1).
// Ref: SRS Req-2.1.1, Req-2.1.2, SAD §A, TASK PR-5.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cascata-platform/cascata/control-plane/internal/config"
	"github.com/cascata-platform/cascata/control-plane/internal/server"
)

func main() {
	// Carregar configuração: YAML + env vars. Zero hardcoded.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[cascata-cp] FATAL: config.Load: %v", err)
	}

	// Context raiz com cancelamento — toda goroutine recebe este ctx.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown: SIGINT (Ctrl+C) ou SIGTERM (docker stop)
	go handleShutdown(cancel)

	// Criar e iniciar o server
	srv := server.New(cfg)

	// Executar — bloqueia até ctx ser cancelado
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("[cascata-cp] FATAL: server.Run: %v", err)
	}
}

// handleShutdown escuta sinais do OS e cancela o context raiz.
// Executado como goroutine — recebe o cancel do context.
func handleShutdown(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("[cascata-cp] signal received: %s — initiating shutdown", sig)
	cancel()
}
