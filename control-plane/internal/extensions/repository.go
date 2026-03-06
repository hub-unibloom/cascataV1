// Package extensions — Data Access Layer 
// Ref: SRS Req-2.20.3, Req-2.20.5
// Ref: TASK_Fase0_Alicerce_Invisivel 0.8.2 e 0.8.3
package extensions

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ExtensionCatalog entry mapeia as informações brutas baseadas na Seed (0.8.1).
type ExtensionCatalog struct {
	Name              string
	DisplayName       string
	Description       string
	Category          int
	CompatLevel       string
	CompatNotes       string
	UsageSnippet      string
	BlockedReason     string
	StorageImpact     string
	PerformanceImpact string
	AvailableInShared bool
}

// TenantExtension map entry na cascata_cp.tenant_extensions.
type TenantExtension struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Extension string `json:"extension"`
	Status    string `json:"status"` // pending, active
	EnabledBy string `json:"enabled_by"`
}

// ExtensionMarketplaceItem representa a formatação DTO para entrega do Painel Web.
type ExtensionMarketplaceItem struct {
	Name              string `json:"name"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	Category          int    `json:"category"`
	CompatLevel       string `json:"compat_level"`
	CompatNotes       string `json:"compat_notes"`
	UsageSnippet      string `json:"usage_snippet"`
	BlockedReason     string `json:"blocked_reason"`
	StorageImpact     string `json:"storage_impact"`
	PerformanceImpact string `json:"performance_impact"`
	Status            string `json:"status"` // available, enabled, unavailable, blocked (derived)
	EnabledAt         string `json:"enabled_at,omitempty"`
	EnabledBy         string `json:"enabled_by,omitempty"`
}

// ExtensionRepository lida com banco Control Plane via tx.
type ExtensionRepository struct {
	cpDB *pgxpool.Pool
}

// NewExtensionRepository injeta pool Control Plane Pgx.
func NewExtensionRepository(cpDB *pgxpool.Pool) *ExtensionRepository {
	return &ExtensionRepository{cpDB: cpDB}
}

// GetCatalogEntry valida se uma extension existe de per si (Catálogo Raw).
func (r *ExtensionRepository) GetCatalogEntry(ctx context.Context, extensionName string) (*ExtensionCatalog, error) {
	row := r.cpDB.QueryRow(ctx, `
		SELECT name, display_name, category, available_in_shared, blocked_reason
		FROM cascata_cp.extensions_catalog
		WHERE name = $1
	`, extensionName)

	var ext ExtensionCatalog
	var blocked *string

	err := row.Scan(&ext.Name, &ext.DisplayName, &ext.Category, &ext.AvailableInShared, &blocked)
	if err != nil {
		return nil, err
	}
	if blocked != nil {
		ext.BlockedReason = *blocked
	}

	return &ext, nil
}

// ListMarketplaceForTenant serve a view master formatada para Request WEB. (0.8.2)
func (r *ExtensionRepository) ListMarketplaceForTenant(ctx context.Context, tenantID string) ([]ExtensionMarketplaceItem, error) {
	query := `
		SELECT
			ec.name,
			ec.display_name,
			COALESCE(ec.description, ''),
			ec.category,
			ec.compat_level,
			COALESCE(ec.compat_notes, ''),
			COALESCE(ec.usage_snippet, ''),
			COALESCE(ec.blocked_reason, ''),
			COALESCE(ec.storage_impact, ''),
			COALESCE(ec.performance_impact, ''),
			CASE
				WHEN ec.category = 4                                        THEN 'blocked'
				WHEN te.id IS NOT NULL AND te.status = 'active'             THEN 'enabled'
				WHEN te.id IS NOT NULL AND te.status = 'pending'            THEN 'pending'
				WHEN t.image_variant = 'shared' AND NOT ec.available_in_shared THEN 'unavailable'
				ELSE 'available'
			END AS status,
			COALESCE(te.enabled_at::text, ''),
			COALESCE(te.enabled_by::text, '')
		FROM cascata_cp.extensions_catalog ec
		CROSS JOIN cascata_cp.tenants t
		LEFT JOIN cascata_cp.tenant_extensions te
			ON te.extension = ec.name AND te.tenant_id = t.id
		WHERE t.id = $1
		ORDER BY ec.category, ec.display_name;
	`

	rows, err := r.cpDB.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ExtensionMarketplaceItem
	for rows.Next() {
		var item ExtensionMarketplaceItem
		err := rows.Scan(
			&item.Name, &item.DisplayName, &item.Description, &item.Category, &item.CompatLevel,
			&item.CompatNotes, &item.UsageSnippet, &item.BlockedReason, &item.StorageImpact,
			&item.PerformanceImpact, &item.Status, &item.EnabledAt, &item.EnabledBy,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// InsertTenantExtension executa a "PRIMEIRA ESCRITA" (pending). 
func (r *ExtensionRepository) InsertTenantExtension(ctx context.Context, te TenantExtension) (TenantExtension, error) {
	err := r.cpDB.QueryRow(ctx, `
		INSERT INTO cascata_cp.tenant_extensions (tenant_id, extension, enabled_by, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, te.TenantID, te.Extension, te.EnabledBy, te.Status).Scan(&te.ID)

	return te, err
}

// ConfirmTenantExtension é a marcação de 'active' após o driver responder ok (SEGUNDA ESCRITA).
func (r *ExtensionRepository) ConfirmTenantExtension(ctx context.Context, recordID string) error {
	_, err := r.cpDB.Exec(ctx, `
		UPDATE cascata_cp.tenant_extensions 
		SET status = 'active', enabled_at = now() 
		WHERE id = $1
	`, recordID)
	return err
}

// DeleteTenantExtension cancela o ciclo em caso do PostgreSQL falhar e deleta rastro.
func (r *ExtensionRepository) DeleteTenantExtension(ctx context.Context, tenantID, recordID string) error {
	_, err := r.cpDB.Exec(ctx, `
		DELETE FROM cascata_cp.tenant_extensions 
		WHERE id = $1 AND tenant_id = $2
	`, recordID, tenantID)
	return err
}

// GetStalePendingExtensions busca intends de extensões que travaram no estado "pending" 
// (devido à crashs do worker) há mais tempo do que a janela segura de operação.
// Beneficia-se diretamente do índice parcial idx_tenant_ext_pending criado na Migration 003.
func (r *ExtensionRepository) GetStalePendingExtensions(ctx context.Context, thresholdMinutes time.Duration) ([]TenantExtension, error) {
	// A conversão do thresholdMinutes em SQL Safe cast:
	query := `
		SELECT id, tenant_id, extension, status
		FROM cascata_cp.tenant_extensions
		WHERE status = 'pending'
		  AND created_at < now() - $1::interval
		ORDER BY created_at ASC;
	`
	
	intervalStr := fmt.Sprintf("%d minutes", int(thresholdMinutes.Minutes()))
	rows, err := r.cpDB.Query(ctx, query, intervalStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stale []TenantExtension
	for rows.Next() {
		var te TenantExtension
		if err := rows.Scan(&te.ID, &te.TenantID, &te.Extension, &te.Status); err != nil {
			return nil, err
		}
		stale = append(stale, te)
	}

	return stale, nil
}
