package pool

import (
	"context"
	"math"
)

// TierConfigs armazena os constraints de cada tier
type TierConfig struct {
	MinPoolSize      int
	SafetyFactor     float64
	QueueSizeLimit   int
	StatementTimeout int
	IdleTimeout      int
}

var Tiers = map[string]TierConfig{
	"NANO":       {MinPoolSize: 3, SafetyFactor: 1.2, QueueSizeLimit: 50, StatementTimeout: 10000, IdleTimeout: 5000},
	"MICRO":      {MinPoolSize: 5, SafetyFactor: 1.3, QueueSizeLimit: 100, StatementTimeout: 30000, IdleTimeout: 15000},
	"STANDARD":   {MinPoolSize: 10, SafetyFactor: 1.5, QueueSizeLimit: 500, StatementTimeout: 60000, IdleTimeout: 30000},
	"ENTERPRISE": {MinPoolSize: 25, SafetyFactor: 2.0, QueueSizeLimit: 2000, StatementTimeout: 300000, IdleTimeout: 120000},
}

// CalculateAdaptiveSize implementa a fórmula do Req-3.5.12
// pool_size = max(tier_min, ceil(p95_concurrent_7d * safety_factor * (1 + growth_trend)))
func CalculateAdaptiveSize(tier string, p95Concurrent float64, growthTrend float64) int {
	config, ok := Tiers[tier]
	if !ok {
		config = Tiers["NANO"] // Fallback seguro
	}

	calculated := math.Ceil(p95Concurrent * config.SafetyFactor * (1.0 + growthTrend))

	if int(calculated) > config.MinPoolSize {
		return int(calculated)
	}
	return config.MinPoolSize
}

// AdaptiveSizingJob simula o worker do Control Plane que recalcula 
// e aplica on-the-fly via Orchestrator
func AdaptiveSizingJob(ctx context.Context, orch *Orchestrator) error {
	// 1. Idealmente buscamos dados p95 e trend do ClickHouse...
	
	// 2. Calculamos Size
	
	// 3. orch.UpdatePool com Reload
	return nil
}
