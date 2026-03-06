// Package metadata — Schema Metadata CRUD no YugabyteDB do Control Plane.
// Todas as queries com LIMIT explícito (Regra 3.2).
// Erros propagados com contexto (Regra 5.1).
// Ref: SRS Req-2.17.7, SAD §Schema Metadata, TASK 0.4/0.5/0.6.
package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaStore gerencia operações CRUD no schema metadata do Control Plane.
// Conexão ao banco do Control Plane (cascata_cp), NÃO ao banco do tenant.
type SchemaStore struct {
	db *sql.DB
}

// NewSchemaStore cria uma nova instância conectada ao banco do Control Plane.
func NewSchemaStore(db *sql.DB) *SchemaStore {
	return &SchemaStore{db: db}
}

// --- TABLE METADATA ---

// GetTable busca uma tabela pelo ID.
func (s *SchemaStore) GetTable(ctx context.Context, tableID string) (*TableMeta, error) {
	const query = `
		SELECT id, tenant_id, table_name, table_schema, status,
		       original_name, original_schema, recycled_name,
		       deleted_at, deleted_by, scheduled_purge_at,
		       row_count_at_deletion, suspended_fkeys, impact_analysis,
		       created_at, updated_at
		FROM cascata_cp.table_metadata
		WHERE id = $1
		LIMIT 1`

	var t TableMeta
	var suspendedFkeys, impactAnalysis sql.NullString

	err := s.db.QueryRowContext(ctx, query, tableID).Scan(
		&t.ID, &t.TenantID, &t.TableName, &t.TableSchema, &t.Status,
		&t.OriginalName, &t.OriginalSchema, &t.RecycledName,
		&t.DeletedAt, &t.DeletedBy, &t.ScheduledPurgeAt,
		&t.RowCountAtDel, &suspendedFkeys, &impactAnalysis,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("metadata.GetTable(%s): %w", tableID, err)
	}

	if suspendedFkeys.Valid {
		t.SuspendedFKeys = json.RawMessage(suspendedFkeys.String)
	}
	if impactAnalysis.Valid {
		t.ImpactAnalysis = json.RawMessage(impactAnalysis.String)
	}

	return &t, nil
}

// GetTableByName busca uma tabela ativa pelo nome e tenant.
func (s *SchemaStore) GetTableByName(ctx context.Context, tenantID, tableName, tableSchema string) (*TableMeta, error) {
	const query = `
		SELECT id, tenant_id, table_name, table_schema, status,
		       original_name, original_schema, recycled_name,
		       deleted_at, deleted_by, scheduled_purge_at,
		       row_count_at_deletion, suspended_fkeys, impact_analysis,
		       created_at, updated_at
		FROM cascata_cp.table_metadata
		WHERE tenant_id = $1 AND table_name = $2 AND table_schema = $3
		LIMIT 1`

	var t TableMeta
	var suspendedFkeys, impactAnalysis sql.NullString

	err := s.db.QueryRowContext(ctx, query, tenantID, tableName, tableSchema).Scan(
		&t.ID, &t.TenantID, &t.TableName, &t.TableSchema, &t.Status,
		&t.OriginalName, &t.OriginalSchema, &t.RecycledName,
		&t.DeletedAt, &t.DeletedBy, &t.ScheduledPurgeAt,
		&t.RowCountAtDel, &suspendedFkeys, &impactAnalysis,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("metadata.GetTableByName(%s, %s.%s): %w", tenantID, tableSchema, tableName, err)
	}

	if suspendedFkeys.Valid {
		t.SuspendedFKeys = json.RawMessage(suspendedFkeys.String)
	}
	if impactAnalysis.Valid {
		t.ImpactAnalysis = json.RawMessage(impactAnalysis.String)
	}

	return &t, nil
}

// ListActiveTables lista tabelas ativas de um tenant com paginação.
func (s *SchemaStore) ListActiveTables(ctx context.Context, tenantID string, limit, offset int) ([]TableMeta, error) {
	if limit <= 0 || limit > 500 {
		limit = 100 // Default seguro
	}

	const query = `
		SELECT id, tenant_id, table_name, table_schema, status,
		       created_at, updated_at
		FROM cascata_cp.table_metadata
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY table_name
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("metadata.ListActiveTables(%s): %w", tenantID, err)
	}
	defer rows.Close()

	var tables []TableMeta
	for rows.Next() {
		var t TableMeta
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.TableName, &t.TableSchema, &t.Status,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("metadata.ListActiveTables scan: %w", err)
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// ListRecycledTables lista tabelas recicladas de um tenant com paginação.
func (s *SchemaStore) ListRecycledTables(ctx context.Context, tenantID string, limit, offset int) ([]TableMeta, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	const query = `
		SELECT id, tenant_id, table_name, table_schema, status,
		       original_name, original_schema, recycled_name,
		       deleted_at, deleted_by, scheduled_purge_at,
		       row_count_at_deletion,
		       created_at, updated_at
		FROM cascata_cp.table_metadata
		WHERE tenant_id = $1 AND status = 'recycled'
		ORDER BY deleted_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("metadata.ListRecycledTables(%s): %w", tenantID, err)
	}
	defer rows.Close()

	var tables []TableMeta
	for rows.Next() {
		var t TableMeta
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.TableName, &t.TableSchema, &t.Status,
			&t.OriginalName, &t.OriginalSchema, &t.RecycledName,
			&t.DeletedAt, &t.DeletedBy, &t.ScheduledPurgeAt,
			&t.RowCountAtDel,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("metadata.ListRecycledTables scan: %w", err)
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// ListExpiredRecycledTables lista tabelas recicladas que venceram a retenção.
// Usada pelo PurgeScheduler para purge automático (Req-2.17.4).
func (s *SchemaStore) ListExpiredRecycledTables(ctx context.Context, limit int) ([]TableMeta, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const query = `
		SELECT id, tenant_id, table_name, table_schema, status,
		       original_name, original_schema, recycled_name,
		       deleted_at, deleted_by, scheduled_purge_at,
		       row_count_at_deletion,
		       created_at, updated_at
		FROM cascata_cp.table_metadata
		WHERE status = 'recycled'
		  AND scheduled_purge_at <= now()
		ORDER BY scheduled_purge_at
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("metadata.ListExpiredRecycledTables: %w", err)
	}
	defer rows.Close()

	var tables []TableMeta
	for rows.Next() {
		var t TableMeta
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.TableName, &t.TableSchema, &t.Status,
			&t.OriginalName, &t.OriginalSchema, &t.RecycledName,
			&t.DeletedAt, &t.DeletedBy, &t.ScheduledPurgeAt,
			&t.RowCountAtDel,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("metadata.ListExpiredRecycledTables scan: %w", err)
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// UpdateTableToRecycled atualiza o metadata de uma tabela para status 'recycled'.
// Chamado pelo Recycler durante a operação de soft delete (Req-2.17.1).
func (s *SchemaStore) UpdateTableToRecycled(ctx context.Context, tableID string, recycledName string, deletedBy string, rowCount int64, scheduledPurge time.Time, suspendedFKs []ForeignKeyInfo, impact *ImpactAnalysis) error {
	fksJSON, err := json.Marshal(suspendedFKs)
	if err != nil {
		return fmt.Errorf("metadata.UpdateTableToRecycled: marshal FKs: %w", err)
	}

	impactJSON, err := json.Marshal(impact)
	if err != nil {
		return fmt.Errorf("metadata.UpdateTableToRecycled: marshal impact: %w", err)
	}

	const query = `
		UPDATE cascata_cp.table_metadata
		SET status = 'recycled',
		    original_name = table_name,
		    original_schema = table_schema,
		    recycled_name = $2,
		    deleted_at = now(),
		    deleted_by = $3,
		    scheduled_purge_at = $4,
		    row_count_at_deletion = $5,
		    suspended_fkeys = $6,
		    impact_analysis = $7,
		    updated_at = now()
		WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, tableID, recycledName, deletedBy, scheduledPurge, rowCount, string(fksJSON), string(impactJSON))
	if err != nil {
		return fmt.Errorf("metadata.UpdateTableToRecycled(%s): %w", tableID, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metadata.UpdateTableToRecycled(%s): tabela não encontrada", tableID)
	}

	return nil
}

// UpdateTableToActive restaura o metadata de uma tabela para status 'active'.
// Chamado pelo Restorer durante a restauração (Req-2.17.5).
func (s *SchemaStore) UpdateTableToActive(ctx context.Context, tableID string, restoredName, restoredSchema string) error {
	const query = `
		UPDATE cascata_cp.table_metadata
		SET status = 'active',
		    table_name = $2,
		    table_schema = $3,
		    original_name = NULL,
		    original_schema = NULL,
		    recycled_name = NULL,
		    deleted_at = NULL,
		    deleted_by = NULL,
		    scheduled_purge_at = NULL,
		    row_count_at_deletion = NULL,
		    suspended_fkeys = '[]'::jsonb,
		    impact_analysis = NULL,
		    updated_at = now()
		WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, tableID, restoredName, restoredSchema)
	if err != nil {
		return fmt.Errorf("metadata.UpdateTableToActive(%s): %w", tableID, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metadata.UpdateTableToActive(%s): tabela não encontrada", tableID)
	}

	return nil
}

// DeleteTableMeta remove permanentemente o metadata de uma tabela.
// Chamado pelo Purger durante o purge permanente (Req-2.17.6).
// column_metadata é deletado via ON DELETE CASCADE.
func (s *SchemaStore) DeleteTableMeta(ctx context.Context, tableID string) error {
	const query = `DELETE FROM cascata_cp.table_metadata WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, tableID)
	if err != nil {
		return fmt.Errorf("metadata.DeleteTableMeta(%s): %w", tableID, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metadata.DeleteTableMeta(%s): tabela não encontrada", tableID)
	}

	return nil
}

// --- COLUMN METADATA ---

// GetColumnsForTable busca todas as colunas ativas de uma tabela.
func (s *SchemaStore) GetColumnsForTable(ctx context.Context, tableMetaID string) ([]ColumnMeta, error) {
	const query = `
		SELECT id, table_meta_id, tenant_id, column_name, data_type,
		       is_nullable, default_value, ordinal_pos, status,
		       computation, validations, pii, is_writable,
		       created_at, updated_at
		FROM cascata_cp.column_metadata
		WHERE table_meta_id = $1 AND status = 'active'
		ORDER BY ordinal_pos
		LIMIT 500`

	rows, err := s.db.QueryContext(ctx, query, tableMetaID)
	if err != nil {
		return nil, fmt.Errorf("metadata.GetColumnsForTable(%s): %w", tableMetaID, err)
	}
	defer rows.Close()

	var columns []ColumnMeta
	for rows.Next() {
		var c ColumnMeta
		var computationJSON, validationsJSON sql.NullString

		if err := rows.Scan(
			&c.ID, &c.TableMetaID, &c.TenantID, &c.ColumnName, &c.DataType,
			&c.IsNullable, &c.DefaultVal, &c.OrdinalPos, &c.Status,
			&computationJSON, &validationsJSON, &c.PII, &c.IsWritable,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("metadata.GetColumnsForTable scan: %w", err)
		}

		if computationJSON.Valid && computationJSON.String != "" {
			var comp Computation
			if err := json.Unmarshal([]byte(computationJSON.String), &comp); err != nil {
				return nil, fmt.Errorf("metadata.GetColumnsForTable unmarshal computation(%s): %w", c.ColumnName, err)
			}
			c.Computation = &comp
		}

		if validationsJSON.Valid && validationsJSON.String != "" && validationsJSON.String != "[]" {
			if err := json.Unmarshal([]byte(validationsJSON.String), &c.Validations); err != nil {
				return nil, fmt.Errorf("metadata.GetColumnsForTable unmarshal validations(%s): %w", c.ColumnName, err)
			}
		}

		columns = append(columns, c)
	}

	return columns, rows.Err()
}

// GetComputedColumns busca apenas colunas com computation definida para um tenant.
// Usado pelo Protocol Cascata para análise de dependências (Req-2.18.4).
func (s *SchemaStore) GetComputedColumns(ctx context.Context, tenantID string) ([]ColumnMeta, error) {
	const query = `
		SELECT cm.id, cm.table_meta_id, cm.tenant_id, cm.column_name, cm.data_type,
		       cm.is_nullable, cm.default_value, cm.ordinal_pos, cm.status,
		       cm.computation, cm.validations, cm.pii, cm.is_writable,
		       cm.created_at, cm.updated_at
		FROM cascata_cp.column_metadata cm
		JOIN cascata_cp.table_metadata tm ON cm.table_meta_id = tm.id
		WHERE cm.tenant_id = $1
		  AND cm.status = 'active'
		  AND tm.status = 'active'
		  AND cm.computation IS NOT NULL
		ORDER BY cm.column_name
		LIMIT 500`

	rows, err := s.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("metadata.GetComputedColumns(%s): %w", tenantID, err)
	}
	defer rows.Close()

	var columns []ColumnMeta
	for rows.Next() {
		var c ColumnMeta
		var computationJSON, validationsJSON sql.NullString

		if err := rows.Scan(
			&c.ID, &c.TableMetaID, &c.TenantID, &c.ColumnName, &c.DataType,
			&c.IsNullable, &c.DefaultVal, &c.OrdinalPos, &c.Status,
			&computationJSON, &validationsJSON, &c.PII, &c.IsWritable,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("metadata.GetComputedColumns scan: %w", err)
		}

		if computationJSON.Valid {
			var comp Computation
			if err := json.Unmarshal([]byte(computationJSON.String), &comp); err != nil {
				return nil, fmt.Errorf("metadata.GetComputedColumns unmarshal(%s): %w", c.ColumnName, err)
			}
			c.Computation = &comp
		}

		columns = append(columns, c)
	}

	return columns, rows.Err()
}

// --- VALIDATION RULES CRUD ---
// Métodos para gerenciar regras de validação por coluna.
// Toda modificação invalida o cache DragonflyDB via MetadataCache.
// Ref: SRS Req-2.19.4, SAD §Validation Engine, TASK 0.6.

// AddValidationRule adiciona uma regra de validação a uma coluna.
// A regra é adicionada ao final do array JSONB `validations`.
// Retorna erro se a coluna não for encontrada ou se a serialização falhar.
func (s *SchemaStore) AddValidationRule(ctx context.Context, columnID string, rule ValidationRule) error {
	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("metadata.AddValidationRule: marshal rule: %w", err)
	}

	// Usa jsonb_insert ou concatenação para adicionar ao array
	const query = `
		UPDATE cascata_cp.column_metadata
		SET validations = validations || $2::jsonb,
		    updated_at = now()
		WHERE id = $1`

	// Wrap em array para concatenação correta: validations || '[{rule}]'
	arrayJSON := fmt.Sprintf("[%s]", string(ruleJSON))

	result, err := s.db.ExecContext(ctx, query, columnID, arrayJSON)
	if err != nil {
		return fmt.Errorf("metadata.AddValidationRule(%s): %w", columnID, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metadata.AddValidationRule(%s): coluna não encontrada", columnID)
	}

	return nil
}

// UpdateValidationRule atualiza uma regra de validação em uma posição específica.
// O ruleIndex é 0-based. Retorna erro se o índice estiver fora dos limites.
func (s *SchemaStore) UpdateValidationRule(ctx context.Context, columnID string, ruleIndex int, rule ValidationRule) error {
	if ruleIndex < 0 {
		return fmt.Errorf("metadata.UpdateValidationRule: index negativo: %d", ruleIndex)
	}

	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("metadata.UpdateValidationRule: marshal rule: %w", err)
	}

	// Usa jsonb_set para atualizar no índice específico
	// jsonb_set(validations, '{index}', new_value)
	const query = `
		UPDATE cascata_cp.column_metadata
		SET validations = jsonb_set(validations, $3::text[], $2::jsonb),
		    updated_at = now()
		WHERE id = $1
		  AND jsonb_array_length(validations) > $4`

	path := fmt.Sprintf("{%d}", ruleIndex)

	result, err := s.db.ExecContext(ctx, query, columnID, string(ruleJSON), path, ruleIndex)
	if err != nil {
		return fmt.Errorf("metadata.UpdateValidationRule(%s, %d): %w", columnID, ruleIndex, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metadata.UpdateValidationRule(%s, %d): coluna não encontrada ou índice fora dos limites", columnID, ruleIndex)
	}

	return nil
}

// RemoveValidationRule remove uma regra de validação em uma posição específica.
// O ruleIndex é 0-based. Retorna erro se o índice estiver fora dos limites.
func (s *SchemaStore) RemoveValidationRule(ctx context.Context, columnID string, ruleIndex int) error {
	if ruleIndex < 0 {
		return fmt.Errorf("metadata.RemoveValidationRule: index negativo: %d", ruleIndex)
	}

	// Remove elemento no índice usando operador - (jsonb minus by index)
	const query = `
		UPDATE cascata_cp.column_metadata
		SET validations = validations - $2,
		    updated_at = now()
		WHERE id = $1
		  AND jsonb_array_length(validations) > $2`

	result, err := s.db.ExecContext(ctx, query, columnID, ruleIndex)
	if err != nil {
		return fmt.Errorf("metadata.RemoveValidationRule(%s, %d): %w", columnID, ruleIndex, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metadata.RemoveValidationRule(%s, %d): coluna não encontrada ou índice fora dos limites", columnID, ruleIndex)
	}

	return nil
}

// GetValidationRulesForTable retorna todas as regras de validação de uma tabela
// no formato flat consumido pelo Pingora (FlatValidationRule).
// Cada regra inclui o field name para que o Pingora não precise cruzar com column_metadata.
// Usado para popular o cache DragonflyDB que o Validation Engine consome.
// Ref: SRS Req-2.19.4, SAD §Validation Engine.
func (s *SchemaStore) GetValidationRulesForTable(ctx context.Context, tableMetaID string) ([]FlatValidationRule, error) {
	const query = `
		SELECT column_name, validations
		FROM cascata_cp.column_metadata
		WHERE table_meta_id = $1
		  AND status = 'active'
		  AND validations != '[]'::jsonb
		  AND validations IS NOT NULL
		ORDER BY ordinal_pos
		LIMIT 500`

	rows, err := s.db.QueryContext(ctx, query, tableMetaID)
	if err != nil {
		return nil, fmt.Errorf("metadata.GetValidationRulesForTable(%s): %w", tableMetaID, err)
	}
	defer rows.Close()

	var flatRules []FlatValidationRule
	for rows.Next() {
		var columnName string
		var validationsJSON string

		if err := rows.Scan(&columnName, &validationsJSON); err != nil {
			return nil, fmt.Errorf("metadata.GetValidationRulesForTable scan: %w", err)
		}

		if validationsJSON == "" || validationsJSON == "[]" {
			continue
		}

		var rules []ValidationRule
		if err := json.Unmarshal([]byte(validationsJSON), &rules); err != nil {
			return nil, fmt.Errorf("metadata.GetValidationRulesForTable unmarshal(%s): %w", columnName, err)
		}

		for _, rule := range rules {
			flatRules = append(flatRules, FlatValidationRule{
				Field:    columnName,
				RuleType: rule.Type,
				Params:   rule.Params,
				Message:  rule.Message,
				Severity: rule.Severity,
			})
		}
	}

	return flatRules, rows.Err()
}
