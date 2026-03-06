// Package recyclebin — Operação de soft delete (move para _recycled).
// Transação atômica completa conforme SRS Req-2.17.1 e Req-2.17.2.
// Ref: SRS Req-2.17.1, Req-2.17.2, SAD §Soft Delete, TASK 0.4.2.
package recyclebin

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cascata-platform/cascata/control-plane/internal/metadata"
)

// RecycleTable executa a operação atômica de soft delete:
//  1. Count rows para audit
//  2. Análise de impacto (Protocol Cascata)
//  3. Suspender FKs que referenciam esta tabela
//  4. ALTER TABLE SET SCHEMA _recycled
//  5. Rename com timestamp + hash curto
//  6. Aplicar RLS DENY ALL na tabela reciclada
//  7. Atualizar metadata no CP: status='recycled'
//
// tenantDB: conexão privilegiada ao banco do TENANT (não API role).
// cpDB: conexão ao banco do CONTROL PLANE.
// Ref: SRS Req-2.17.1, Req-2.17.2.
func RecycleTable(
	ctx context.Context,
	tenantDB *sql.DB,
	cpDB *sql.DB,
	cache *metadata.MetadataCache,
	tenantID string,
	tableMetaID string,
	tableName string,
	tableSchema string,
	deletedBy string,
	tier string,
) error {
	// Calcular nome reciclado: {nome}__{timestamp}__{hash_curto}
	now := time.Now()
	hash := md5.Sum([]byte(tableMetaID))
	hashShort := hex.EncodeToString(hash[:])[:4]
	recycledName := fmt.Sprintf("%s__%d__%s", tableName, now.Unix(), hashShort)

	// Calcular data de purge baseada na retenção do tier (Req-2.17.4)
	retConfig, exists := metadata.TierRetention[tier]
	if !exists {
		retConfig = metadata.TierRetention["NANO"] // Fallback seguro
	}
	scheduledPurge := now.AddDate(0, 0, retConfig.DefaultDays)

	// 1. Análise de impacto (Protocol Cascata — Req-2.17.3)
	impact, err := AnalyzeImpact(ctx, tenantDB, cpDB, tenantID, tableName, tableSchema)
	if err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: análise de impacto: %w", err)
	}

	// 2. Transação atômica no banco do TENANT
	tx, err := tenantDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 2a. Contar linhas para audit trail
	var rowCount int64
	countQuery := fmt.Sprintf("SELECT count(*) FROM %s.%s", quoteIdent(tableSchema), quoteIdent(tableName))
	if err = tx.QueryRowContext(ctx, countQuery).Scan(&rowCount); err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: count rows %s.%s: %w", tableSchema, tableName, err)
	}

	// 2b. Suspender foreign keys que referenciam esta tabela
	suspendedFKs := make([]metadata.ForeignKeyInfo, 0, len(impact.ForeignKeys))
	for _, fk := range impact.ForeignKeys {
		dropFK := fmt.Sprintf(
			"ALTER TABLE %s.%s DROP CONSTRAINT %s",
			quoteIdent(fk.ReferencingSchema),
			quoteIdent(fk.ReferencingTable),
			quoteIdent(fk.ConstraintName),
		)
		if _, err = tx.ExecContext(ctx, dropFK); err != nil {
			return fmt.Errorf("recyclebin.RecycleTable: drop FK %s: %w", fk.ConstraintName, err)
		}
		suspendedFKs = append(suspendedFKs, fk)
	}

	// 2c. Mover tabela para schema _recycled
	moveQuery := fmt.Sprintf(
		"ALTER TABLE %s.%s SET SCHEMA _recycled",
		quoteIdent(tableSchema), quoteIdent(tableName),
	)
	if _, err = tx.ExecContext(ctx, moveQuery); err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: SET SCHEMA _recycled %s: %w", tableName, err)
	}

	// 2d. Renomear com timestamp e hash
	renameQuery := fmt.Sprintf(
		"ALTER TABLE _recycled.%s RENAME TO %s",
		quoteIdent(tableName), quoteIdent(recycledName),
	)
	if _, err = tx.ExecContext(ctx, renameQuery); err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: rename %s → %s: %w", tableName, recycledName, err)
	}

	// 2e. Aplicar RLS DENY ALL (Req-2.17.2)
	rlsQueries := []string{
		fmt.Sprintf("ALTER TABLE _recycled.%s ENABLE ROW LEVEL SECURITY", quoteIdent(recycledName)),
		fmt.Sprintf("ALTER TABLE _recycled.%s FORCE ROW LEVEL SECURITY", quoteIdent(recycledName)),
		fmt.Sprintf(
			"CREATE POLICY deny_all ON _recycled.%s FOR ALL USING (false)",
			quoteIdent(recycledName),
		),
	}
	for _, q := range rlsQueries {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("recyclebin.RecycleTable: RLS DENY ALL %s: %w", recycledName, err)
		}
	}

	// 2f. Commit da transação no banco do tenant
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: commit: %w", err)
	}

	// 3. Atualizar metadata no Control Plane
	store := metadata.NewSchemaStore(cpDB)
	if err = store.UpdateTableToRecycled(
		ctx, tableMetaID, recycledName, deletedBy,
		rowCount, scheduledPurge, suspendedFKs, impact,
	); err != nil {
		return fmt.Errorf("recyclebin.RecycleTable: update metadata: %w", err)
	}

	// 4. Invalidar cache DragonflyDB (Req-2.17.7)
	if cache != nil {
		if err = cache.InvalidateTableCache(ctx, tenantID); err != nil {
			// Log mas não falha — operação principal já commitou
			fmt.Printf("[WARN] recyclebin.RecycleTable: falha invalidar cache %s: %v\n", tenantID, err)
		}
	}

	// 5. Evento Redpanda/ClickHouse seria publicado aqui
	// TODO: Publicar evento "table_recycled" no Redpanda → ClickHouse

	return nil
}

// quoteIdent escapa um identificador SQL para evitar SQL injection.
// Usa aspas duplas conforme padrão SQL.
func quoteIdent(name string) string {
	// Rejeitar identificadores com aspas duplas embutidas
	for _, c := range name {
		if c == '"' || c == '\x00' {
			return `"invalid_identifier"`
		}
	}
	return `"` + name + `"`
}
