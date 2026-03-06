// Package recyclebin — Protocol Cascata: análise de impacto pré-delete.
// Queries de descoberta executadas ANTES de qualquer operação destrutiva.
// Identifica foreign keys, computed columns, e RLS policies afetadas.
// Ref: SRS Req-2.17.3, Req-2.18.4, TASK 0.4.3.
package recyclebin

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cascata-platform/cascata/control-plane/internal/metadata"
)

// AnalyzeImpact executa o Protocol Cascata: identifica todos os impactos
// de deletar uma tabela antes de executar qualquer operação.
// Resultado é exibido no modal de confirmação do painel (Req-2.17.3).
func AnalyzeImpact(ctx context.Context, tenantDB *sql.DB, cpDB *sql.DB, tenantID, tableName, tableSchema string) (*metadata.ImpactAnalysis, error) {
	impact := &metadata.ImpactAnalysis{
		ForeignKeys:  []metadata.ForeignKeyInfo{},
		ComputedDeps: []metadata.ComputedDepInfo{},
	}

	// 1. Descobrir foreign keys que referenciam esta tabela (no banco do tenant)
	fks, err := discoverForeignKeys(ctx, tenantDB, tableName, tableSchema)
	if err != nil {
		return nil, fmt.Errorf("recyclebin.AnalyzeImpact: discover FKs para %s.%s: %w", tableSchema, tableName, err)
	}
	impact.ForeignKeys = fks

	// 2. Descobrir computed columns que dependem desta tabela (no banco do CP)
	deps, err := discoverComputedDeps(ctx, cpDB, tenantID, tableName)
	if err != nil {
		return nil, fmt.Errorf("recyclebin.AnalyzeImpact: discover computed deps para %s: %w", tableName, err)
	}
	impact.ComputedDeps = deps

	return impact, nil
}

// discoverForeignKeys encontra FKs em outras tabelas que apontam para a tabela alvo.
// Executa no banco do TENANT (não do Control Plane).
func discoverForeignKeys(ctx context.Context, tenantDB *sql.DB, tableName, tableSchema string) ([]metadata.ForeignKeyInfo, error) {
	const query = `
		SELECT
			tc.constraint_name,
			tc.table_schema AS referencing_schema,
			tc.table_name AS referencing_table,
			kcu.column_name AS referencing_column,
			ccu.column_name AS referenced_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
		WHERE ccu.table_name = $1
			AND ccu.table_schema = $2
			AND tc.constraint_type = 'FOREIGN KEY'
		LIMIT 100`

	rows, err := tenantDB.QueryContext(ctx, query, tableName, tableSchema)
	if err != nil {
		return nil, fmt.Errorf("discoverForeignKeys(%s.%s): %w", tableSchema, tableName, err)
	}
	defer rows.Close()

	var fks []metadata.ForeignKeyInfo
	for rows.Next() {
		var fk metadata.ForeignKeyInfo
		if err := rows.Scan(
			&fk.ConstraintName,
			&fk.ReferencingSchema,
			&fk.ReferencingTable,
			&fk.ReferencingColumn,
			&fk.ReferencedColumn,
		); err != nil {
			return nil, fmt.Errorf("discoverForeignKeys scan: %w", err)
		}
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

// discoverComputedDeps encontra colunas computed cujas expressões referenciam
// a tabela ou colunas da tabela alvo (Req-2.18.4).
// Executa no banco do CONTROL PLANE.
func discoverComputedDeps(ctx context.Context, cpDB *sql.DB, tenantID, tableName string) ([]metadata.ComputedDepInfo, error) {
	// Busca colunas computed que mencionam o nome da tabela na expressão.
	// Para stored_generated, a dependência é detectada no banco pelo YugabyteDB.
	// Para api_computed, precisamos parse da expressão aqui.
	const query = `
		SELECT
			tm.table_name,
			cm.column_name,
			cm.computation->>'expression' AS expression,
			cm.computation->>'kind' AS kind
		FROM cascata_cp.column_metadata cm
		JOIN cascata_cp.table_metadata tm ON cm.table_meta_id = tm.id
		WHERE cm.tenant_id = $1
			AND tm.status = 'active'
			AND cm.status = 'active'
			AND cm.computation IS NOT NULL
			AND cm.computation->>'expression' LIKE '%' || $2 || '%'
		LIMIT 100`

	rows, err := cpDB.QueryContext(ctx, query, tenantID, tableName)
	if err != nil {
		return nil, fmt.Errorf("discoverComputedDeps(%s, %s): %w", tenantID, tableName, err)
	}
	defer rows.Close()

	var deps []metadata.ComputedDepInfo
	for rows.Next() {
		var dep metadata.ComputedDepInfo
		if err := rows.Scan(&dep.TableName, &dep.ColumnName, &dep.Expression, &dep.Kind); err != nil {
			return nil, fmt.Errorf("discoverComputedDeps scan: %w", err)
		}
		deps = append(deps, dep)
	}

	return deps, rows.Err()
}

// AnalyzeColumnImpact verifica impacto de deletar/renomear uma coluna específica.
// Usado pelo Protocol Cascata para computed columns (Req-2.18.4).
func AnalyzeColumnImpact(ctx context.Context, cpDB *sql.DB, tenantID, columnName string) ([]metadata.ComputedDepInfo, error) {
	const query = `
		SELECT
			tm.table_name,
			cm.column_name,
			cm.computation->>'expression' AS expression,
			cm.computation->>'kind' AS kind
		FROM cascata_cp.column_metadata cm
		JOIN cascata_cp.table_metadata tm ON cm.table_meta_id = tm.id
		WHERE cm.tenant_id = $1
			AND tm.status = 'active'
			AND cm.status = 'active'
			AND cm.computation IS NOT NULL
			AND cm.computation->>'expression' LIKE '%' || $2 || '%'
		LIMIT 100`

	rows, err := cpDB.QueryContext(ctx, query, tenantID, columnName)
	if err != nil {
		return nil, fmt.Errorf("recyclebin.AnalyzeColumnImpact(%s, %s): %w", tenantID, columnName, err)
	}
	defer rows.Close()

	var deps []metadata.ComputedDepInfo
	for rows.Next() {
		var dep metadata.ComputedDepInfo
		if err := rows.Scan(&dep.TableName, &dep.ColumnName, &dep.Expression, &dep.Kind); err != nil {
			return nil, fmt.Errorf("recyclebin.AnalyzeColumnImpact scan: %w", err)
		}
		deps = append(deps, dep)
	}

	return deps, rows.Err()
}
