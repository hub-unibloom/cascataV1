// Package health — endpoints /health e /ready para o Control Plane.
// /health — liveness: o processo está vivo.
// /ready — readiness: o processo pode aceitar requests (deps OK).
// Ref: SAD §A, TASK PR-5.
package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status representa o estado de saúde do serviço.
type Status struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	UptimeSeconds float64           `json:"uptime_seconds"`
	Components    map[string]string `json:"components,omitempty"`
}

// Checker mantém o estado de saúde e verifica componentes.
type Checker struct {
	startTime  time.Time
	version    string
	mu         sync.RWMutex
	components map[string]string // "yugabytedb" → "ok" / "unreachable"
}

// NewChecker cria um health checker com a versão do serviço.
func NewChecker(version string) *Checker {
	return &Checker{
		startTime:  time.Now(),
		version:    version,
		components: make(map[string]string),
	}
}

// SetComponent atualiza o estado de um componente.
// Thread-safe — pode ser chamado de goroutines de background.
func (c *Checker) SetComponent(name, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.components[name] = status
}

// LiveHandler — GET /health — sempre retorna 200 se o processo está vivo.
func (c *Checker) LiveHandler(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := Status{
		Status:        "ok",
		Version:       c.version,
		UptimeSeconds: time.Since(c.startTime).Seconds(),
		Components:    c.copyComponents(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status) //nolint:errcheck
}

// ReadyHandler — GET /ready — retorna 200 só se todos os componentes estão "ok".
// Se algum componente está degradado, retorna 503.
func (c *Checker) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	allOK := true
	for _, v := range c.components {
		if v != "ok" {
			allOK = false
			break
		}
	}

	statusStr := "ok"
	httpCode := http.StatusOK
	if !allOK {
		statusStr = "degraded"
		httpCode = http.StatusServiceUnavailable
	}

	status := Status{
		Status:        statusStr,
		Version:       c.version,
		UptimeSeconds: time.Since(c.startTime).Seconds(),
		Components:    c.copyComponents(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(status) //nolint:errcheck
}

// copyComponents retorna uma cópia do mapa de componentes.
// Deve ser chamado com lock held.
func (c *Checker) copyComponents() map[string]string {
	if len(c.components) == 0 {
		return nil
	}
	cp := make(map[string]string, len(c.components))
	for k, v := range c.components {
		cp[k] = v
	}
	return cp
}
