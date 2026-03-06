// Package tenant — Provisiona recursos por tier e executa as Propostas do Motor de Classificação.
// Implementação: 0.7 (Downgrade Automático) e PR-9.
package tenant

import (
	"errors"
	"fmt"
)

var (
	ErrDowngradeBlockedActiveSLA      = errors.New("downgrade bloqueado: tier proposto não atende SLA contratado pelo tenant")
	ErrDowngradeBlockedImmutableAudit = errors.New("downgrade bloqueado: tier atual exige auditoria imutável (Incompatível com o alvo)")
	ErrDowngradeBlockedPHIData        = errors.New("downgrade bloqueado: o tenant contém flags de PHI/HIPAA que exigem tier dedicado não presente no alvo")
	ErrDowngradeBlockedExtensions     = errors.New("downgrade bloqueado: extensões incompatíveis habilitadas")
	ErrDowngradeBlockedStorage        = errors.New("downgrade bloqueado: armazenamento consumido excede as cotas do alvo")
	ErrDowngradeBlockedSovereign      = errors.New("downgrade bloqueado: SOVEREIGN não participa da engine de automação de tiers")
)

// ProposalDecision opções do Action Handler (0.7.5).
type ProposalDecision string

const (
	DecisionApprove  ProposalDecision = "approved"  // executa na próxima janela
	DecisionReject   ProposalDecision = "rejected"  // proposta encerrada
	DecisionPostpone ProposalDecision = "postponed" // requer nova data (mínimo 7 dias à frente)
	DecisionDelegate ProposalDecision = "delegate"  // delega a agente autorizado via ABAC
)

// TenantState encapsula o estado contextual para o validador
type TenantState struct {
	CurrentTier   string
	StorageUsedGB float64
	HasActiveSLA  bool
	HasPHIData    bool
}

// Provisioner contém os métodos de execução e restrição de Provisioning Action
type Provisioner struct{}

// tierHasSLA simula uma verificação simples: apenas ENTERPRISE e STANDARD garantem.
func tierHasSLA(tier string) bool {
	return tier == "ENTERPRISE" || tier == "STANDARD"
}

// tierHasImmutableAudit: ENTERPRISE e SOVEREIGN
func tierHasImmutableAudit(tier string) bool {
	return tier == "ENTERPRISE" || tier == "SOVEREIGN"
}

// tierHasDedicatedNamespace: STANDARD, ENTERPRISE, SOVEREIGN
func tierHasDedicatedNamespace(tier string) bool {
	return tier == "STANDARD" || tier == "ENTERPRISE" || tier == "SOVEREIGN"
}

// tierUsesSharedImage: MICRO e NANO
func tierUsesSharedImage(tier string) bool {
	return tier == "MICRO" || tier == "NANO"
}

// tierStorageQuota retorna float
func tierStorageQuota(tier string) float64 {
	switch tier {
	case "NANO":
		return 0.5
	case "MICRO":
		return 2.0
	case "STANDARD":
		return 50.0
	case "ENTERPRISE":
		return 500.0
	default:
		return 99999.0 // Sovereign ou desconhecido
	}
}

// validateDowngradeSafe bloqueia agressivamente o downgrade caso o tenant viole features vitais (0.7.7)
func (p *Provisioner) validateDowngradeSafe(tenantID string, tenant TenantState, proposedTier string, enabledExtensions []string) error {

	// BLOQUEIO 6: SOVEREIGN
	if tenant.CurrentTier == "SOVEREIGN" {
		return ErrDowngradeBlockedSovereign
	}

	// BLOQUEIO 1: SLA ativo
	if tenant.HasActiveSLA && !tierHasSLA(proposedTier) {
		return ErrDowngradeBlockedActiveSLA
	}

	// BLOQUEIO 2: Audit trail imutável
	if tierHasImmutableAudit(tenant.CurrentTier) && !tierHasImmutableAudit(proposedTier) {
		return ErrDowngradeBlockedImmutableAudit
	}

	// BLOQUEIO 3: PHI / dados regulados
	if tenant.HasPHIData && !tierHasDedicatedNamespace(proposedTier) {
		return ErrDowngradeBlockedPHIData
	}

	// BLOQUEIO 4: Extensões exclusivas do tier atual
	if tierUsesSharedImage(proposedTier) && len(enabledExtensions) > 0 {
		return fmt.Errorf("%w: extensões ativas incompatíveis com :shared: %v", ErrDowngradeBlockedExtensions, enabledExtensions)
	}

	// BLOQUEIO 5: Storage excede cota do tier proposto
	if tenant.StorageUsedGB > tierStorageQuota(proposedTier) {
		return fmt.Errorf("%w: storage utilizado (%.1fGB) excede cota do tier %s (%.1fGB)",
			ErrDowngradeBlockedStorage, tenant.StorageUsedGB, proposedTier, tierStorageQuota(proposedTier))
	}

	return nil
}
