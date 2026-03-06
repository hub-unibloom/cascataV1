// Package tenant — Smart Tenant Classification Engine (promote/downgrade).
// Implementação: 0.7 (Downgrade Automático Bidirecional).
package tenant

import (
	"time"
)

// TenantMetrics guarda os dados vitais para decisão de classificação (Req-2.1.1).
// Uma snapshot desta struct é gerada diariamente consultando o ClickHouse.
type TenantMetrics struct {
	TenantID                  string
	Tier                      string
	
	// Tráfego
	P95RequestsPerHourLast30d float64 // p95 de requests/hora nos últimos 30 dias
	P95ConcurrentConnsLast30d float64 // p95 de conexões simultâneas
	
	// Dados
	StorageUsedGB             float64
	
	// Usuários
	ActiveUsersLast30d        int64   // usuários únicos com atividade
	SimultaneousAgentsMax     int64   // max agentes simultâneos ativos
	
	// Atividade
	LastActivityAt            time.Time
	DaysInactiveConsecutive   int     // dias sem qualquer request
	
	// Tendência
	GrowthTrendCoefficient    float64 // slope da regressão linear 30d
}

// InactivityStatus define os três níveis de inatividade de um projeto (0.7.3)
type InactivityStatus string

const (
	InactivityNone   InactivityStatus = "none"
	InactivityLevel1 InactivityStatus = "level_1_notification" // 7 a 14 dias
	InactivityLevel2 InactivityStatus = "level_2_proposal"     // 14 a 30 dias (propõe downgrade)
	InactivityLevel3 InactivityStatus = "level_3_hibernation"  // >30 dias (propõe hibernação)
)

// Classifier encapsula o motor bidirecional.
type Classifier struct{}

// evaluateInactivity sobrepõe as volumetrias e foca em absência total de requisições.
func (c *Classifier) evaluateInactivity(m TenantMetrics) InactivityStatus {
	days := m.DaysInactiveConsecutive

	switch {
	case days >= 30:
		return InactivityLevel3 // Proposta de hibernação + janela de 7 dias
	case days >= 14 && days < 30:
		return InactivityLevel2 // Proposta de downgrade criada para operador
	case days >= 7 && days < 14:
		return InactivityLevel1 // Notificação, sem ação agressiva
	default:
		return InactivityNone
	}
}
