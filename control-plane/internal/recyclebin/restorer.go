// Package recyclebin — Restauração do _recycled para schema original.
// Transação atômica reversa conforme SRS Req-2.17.5.
// Ref: SRS Req-2.17.5, SAD §Soft Delete, TASK 0.4.5.
package recyclebin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cascata-platform/cascata/control-plane/internal/metadata"
)

// RestoreTable restaura uma tabela do schema _recycled para o schema original.
// Operação atômica:
//  1. Verificar conflito de nome
//  2. Remover política DENY ALL
//  3. Renomear ao nome original (ou sufixo _restored se conflito)
//  4. Mover de volta para schema original
//  5. Reativar FKs suspensas
//  6. Atualizar metadata: status='active'
//
// Ref: SRS Req-2.17.5.
func RestoreTable(
	ctx context.Context,
	tenantDB *sql.DB,
	cpDB *sql.DB,
	cache *metadata.MetadataCache,
	tenantID string,
	tableMetaID string,
	conflictStrategy metadata.ConflictStrategy,
) error {
	// 1. Buscar metadata da tabela reciclada
	store := metadata.NewSchemaStore(cpDB)
	tableMeta, err := store.GetTable(ctx, tableMetaID)
	if err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: get metadata %s: %w", tableMetaID, err)
	}

	if tableMeta.Status != "recycled" {
		return fmt.Errorf("recyclebin.RestoreTable: tabela %s não está reciclada (status=%s)", tableMetaID, tableMeta.Status)
	}

	if tableMeta.RecycledName == nil || tableMeta.OriginalName == nil || tableMeta.OriginalSchema == nil {
		return fmt.Errorf("recyclebin.RestoreTable: metadata incompleto para tabela %s", tableMetaID)
	}

	recycledName := *tableMeta.RecycledName
	originalName := *tableMeta.OriginalName
	originalSchema := *tableMeta.OriginalSchema

	// 2. Transação atômica no banco do tenant
	tx, err := tenantDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 2a. Verificar conflito de nome
	var nameConflict bool
	conflictQuery := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)`
	if err = tx.QueryRowContext(ctx, conflictQuery, originalSchema, originalName).Scan(&nameConflict); err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: check name conflict: %w", err)
	}

	restoreName := originalName
	if nameConflict {
		switch conflictStrategy {
		case metadata.ConflictRename:
			restoreName = fmt.Sprintf("%s_restored_%d", originalName, time.Now().Unix())
		case metadata.ConflictOverwrite:
			// Dropar tabela existente (requer confirmação no painel antes de chegar aqui)
			dropQuery := fmt.Sprintf("DROP TABLE %s.%s", quoteIdent(originalSchema), quoteIdent(originalName))
			if _, err = tx.ExecContext(ctx, dropQuery); err != nil {
				return fmt.Errorf("recyclebin.RestoreTable: drop existing table %s.%s: %w", originalSchema, originalName, err)
			}
		default:
			return fmt.Errorf("recyclebin.RestoreTable: conflito de nome para %s.%s e estratégia inválida: %s", originalSchema, originalName, conflictStrategy)
		}
	}

	// 2b. Remover política DENY ALL
	dropPolicy := fmt.Sprintf("DROP POLICY IF EXISTS deny_all ON _recycled.%s", quoteIdent(recycledName))
	if _, err = tx.ExecContext(ctx, dropPolicy); err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: drop policy DENY ALL: %w", err)
	}

	// 2c. Desabilitar RLS forçado
	disableRLS := fmt.Sprintf("ALTER TABLE _recycled.%s DISABLE ROW LEVEL SECURITY", quoteIdent(recycledName))
	if _, err = tx.ExecContext(ctx, disableRLS); err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: disable RLS: %w", err)
	}

	// 2d. Renomear ao nome original (ou com sufixo)
	if recycledName != restoreName {
		renameQuery := fmt.Sprintf(
			"ALTER TABLE _recycled.%s RENAME TO %s",
			quoteIdent(recycledName), quoteIdent(restoreName),
		)
		if _, err = tx.ExecContext(ctx, renameQuery); err != nil {
			return fmt.Errorf("recyclebin.RestoreTable: rename %s → %s: %w", recycledName, restoreName, err)
		}
	}

	// 2e. Mover de volta para schema original
	moveQuery := fmt.Sprintf(
		"ALTER TABLE _recycled.%s SET SCHEMA %s",
		quoteIdent(restoreName), quoteIdent(originalSchema),
	)
	if _, err = tx.ExecContext(ctx, moveQuery); err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: SET SCHEMA %s: %w", originalSchema, err)
	}

	// 2f. Reativar FKs suspensas (Req-2.17.5)
	if tableMeta.SuspendedFKeys != nil && len(tableMeta.SuspendedFKeys) > 2 { // mais que "[]"
		var fks []metadata.ForeignKeyInfo
		if errJSON := json.Unmarshal(tableMeta.SuspendedFKeys, &fks); errJSON == nil {
			for _, fk := range fks {
				addFK := fmt.Sprintf(
					"ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s(%s)",
					quoteIdent(fk.ReferencingSchema),
					quoteIdent(fk.ReferencingTable),
					quoteIdent(fk.ConstraintName),
					quoteIdent(fk.ReferencingColumn),
					quoteIdent(originalSchema),
					quoteIdent(restoreName),
					quoteIdent(fk.ReferencedColumn),
				)
				if _, err = tx.ExecContext(ctx, addFK); err != nil {
					// Log o erro mas não falha a restauração inteira por uma FK
					fmt.Printf("[WARN] recyclebin.RestoreTable: falha reativar FK %s: %v\n", fk.ConstraintName, err)
				}
			}
		}
	}

	// 2g. Commit
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: commit: %w", err)
	}

	// 3. Atualizar metadata no Control Plane
	if err = store.UpdateTableToActive(ctx, tableMetaID, restoreName, originalSchema); err != nil {
		return fmt.Errorf("recyclebin.RestoreTable: update metadata: %w", err)
	}

	// 4. Invalidar cache DragonflyDB
	if cache != nil {
		if errCache := cache.InvalidateTableCache(ctx, tenantID); errCache != nil {
			fmt.Printf("[WARN] recyclebin.RestoreTable: falha invalidar cache %s: %v\n", tenantID, errCache)
		}
	}

	// 5. Evento Redpanda
	// TODO: Publicar evento "table_restored" no Redpanda → ClickHouse

	return nil
}
