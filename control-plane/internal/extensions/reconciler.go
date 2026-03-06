// Package extensions — Job de Reconciliação de Estado (0.8.8)
// Ref: SRS Req-2.20.3, SAD Extension Profile System
package extensions

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// stalePendingThreshold — Gap no tempo indicando morte do processo.
const stalePendingThreshold = 5 // em minutos

// Reconciler atua como worker de consolidação para transações quebradas na Pipeline de Ativação.
type Reconciler struct {
	repo     *ExtensionRepository
	tenantDB *pgxpool.Pool // Virtualizado para checar a base do projeto
	events   EventPublisher
	log      Logger
}

func NewReconciler(repo *ExtensionRepository, tenantDB *pgxpool.Pool, events EventPublisher, logger Logger) *Reconciler {
	return &Reconciler{
		repo:     repo,
		tenantDB: tenantDB,
		events:   events,
		log:      logger,
	}
}

// ReconcileExtensions deve ser amarrado ao cron interno do Control Plane a cada 6 horas.
// Sem full scans: o banco aponta as anomalias diretas através do Índice Parcial O(1).
func (r *Reconciler) ReconcileExtensions(ctx context.Context) {
	stale, err := r.repo.GetStalePendingExtensions(ctx, stalePendingThreshold)
	if err != nil {
		r.log.Warn("reconciler_query_failed", "error", err)
		// Falha silenciosa contida. Irá persistir no retry automático (ciclo seguinte).
		return
	}

	if len(stale) == 0 {
		return // Tudo limpo.
	}

	r.log.Warn("reconciler_found_stale_pending", "count", len(stale))

	for _, record := range stale {
		r.reconcileRecord(ctx, record)
	}
}

func (r *Reconciler) reconcileRecord(ctx context.Context, record TenantExtension) {
	// Verificar no banco YugabyteDB do tenant se a query extenuante de fato completou (segunda fase)
	query := `SELECT 1 FROM pg_extension WHERE extname = $1`
	var exists int

	// Simulamos o fetch no schema "public" ou "tenant_uuid" base.
	err := r.tenantDB.QueryRow(ctx, query, record.Extension).Scan(&exists)
	
	if err != nil && err.Error() != "no rows in result set" {
		// YugabyteDB inacessível. Nós pulamos a reconciliação (não tomar decisão na escuridão).
		r.log.Warn("reconciler_tenant_db_unreachable",
			"tenant_id", record.TenantID,
			"extension", record.Extension,
			"record_id", record.ID,
		)
		return
	}

	dbHasExtension := (exists == 1)

	if dbHasExtension {
		// CREATE EXTENSION foi exitoso — o Control Plane Crashou antes do UPDATE 'active'.
		// Vamos honrar a consolidação e destravar a interface do Painel.
		if err := r.repo.ConfirmTenantExtension(ctx, record.ID); err != nil {
			r.log.Warn("reconciler_confirmation_failed", "record_id", record.ID, "error", err)
			return
		}

		r.log.Warn("reconciler_promoted_to_active", "tenant_id", record.TenantID, "extension", record.Extension)
		r.publishReconciliationEvent(ctx, record, "promoted")

	} else {
		// CREATE EXTENSION NÃO executou ou falhou sem apagar o Pending (Crash Kernel).
		// Intenção morreu. Deleta do banco do Control Plane e deixa que a interface libere botao "Habilitar" novamente.
		if err := r.repo.DeleteTenantExtension(ctx, record.TenantID, record.ID); err != nil {
			r.log.Warn("reconciler_cleanup_failed", "record_id", record.ID, "error", err)
			return
		}

		r.log.Warn("reconciler_cleaned_stale_pending", "tenant_id", record.TenantID, "extension", record.Extension)
		r.publishReconciliationEvent(ctx, record, "cleaned")
	}
}

func (r *Reconciler) publishReconciliationEvent(ctx context.Context, record TenantExtension, outcome string) {
	_ = r.events.Publish(ctx, ExtensionEvent{
		TenantID:      record.TenantID,
		Extension:     record.Extension,
		Action:        "reconciled",
		Result:        outcome, // "promoted" ou "cleaned"
		ExecutedBy:    "system_reconciler",
		FailureReason: fmt.Sprintf("Stale Pending Intercom (%s)", outcome), 
	})
}
