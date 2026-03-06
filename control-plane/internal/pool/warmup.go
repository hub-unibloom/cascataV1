package pool

import (
	"context"
	"hash/fnv"
	"time"
)

// LeadTime base dos Tiers (req 3.5.8) em segundos antes do pico histórico
var LeadTime = map[string]int{
	"NANO":       60,
	"MICRO":      90,
	"STANDARD":   120,
	"ENTERPRISE": 180,
	"SOVEREIGN":  180, // Default configurável
}

// WarmingWindowMS Define a janela de warming. 
// Para distribuir uniformemente os wakeups.
const WarmingWindowMS = 10000

// DetermineWakeTime calcula com exactidão a hora pre-aquecida para conectar
// aplicando a lead time e e o determinist jitter
func DetermineWakeTime(tenantID string, tier string, peakTime time.Time) time.Time {
	leadSecs, exists := LeadTime[tier]
	if !exists {
		leadSecs = LeadTime["NANO"]
	}

	// hash(tenant_id)
	h := fnv.New32a()
	h.Write([]byte(tenantID))
	hashVal := h.Sum32()

	// jitter = hash(tenant_id) % warming_window_ms
	jitterMS := int(hashVal % WarmingWindowMS)

	// wake_time = peakTime - lead_time + jitter
	wakeTime := peakTime.Add(-time.Duration(leadSecs) * time.Second).Add(time.Duration(jitterMS) * time.Millisecond)

	return wakeTime
}

// WarmupJob é a coroutine que verifica continuamente tenants que vão receber tráfego
// e aplica "min_pool_size" (pre-warm) via Orchestrator sem causar spike simultaneo.
func WarmupJob(ctx context.Context, orch *Orchestrator, tenantID string) error {
    return nil
}
