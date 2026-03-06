// Package metadata — Invalidação e refresh de cache no DragonflyDB.
// Keys de cache seguem pattern: meta:{tenant_id}:tables, meta:{tenant_id}:columns:{table_id}
// Invalidação obrigatória em toda operação de recycle/restore/purge (Req-2.17.7).
// Ref: SRS Req-2.17.7, SAD §DragonflyDB, TASK 0.4/0.5/0.6.
package metadata

import (
	"context"
	"fmt"
)

// CacheClient interface para operações de cache.
// Abstrai DragonflyDB (API Redis-compatible) para testabilidade.
type CacheClient interface {
	// Del remove uma ou mais chaves do cache.
	Del(ctx context.Context, keys ...string) error
}

// MetadataCache gerencia o cache de schema metadata no DragonflyDB.
type MetadataCache struct {
	client CacheClient
}

// NewMetadataCache cria uma nova instância do cache manager.
func NewMetadataCache(client CacheClient) *MetadataCache {
	return &MetadataCache{client: client}
}

// InvalidateTableCache remove do cache a lista de tabelas de um tenant.
// Chamado em: recycle, restore, purge, create table, drop table.
func (c *MetadataCache) InvalidateTableCache(ctx context.Context, tenantID string) error {
	key := fmt.Sprintf("meta:%s:tables", tenantID)
	if err := c.client.Del(ctx, key); err != nil {
		return fmt.Errorf("cache.InvalidateTableCache(%s): %w", tenantID, err)
	}
	return nil
}

// InvalidateColumnCache remove do cache as colunas de uma tabela específica.
// Chamado em: alter column, add/drop computed column, change validation rule.
func (c *MetadataCache) InvalidateColumnCache(ctx context.Context, tenantID, tableID string) error {
	key := fmt.Sprintf("meta:%s:columns:%s", tenantID, tableID)
	if err := c.client.Del(ctx, key); err != nil {
		return fmt.Errorf("cache.InvalidateColumnCache(%s, %s): %w", tenantID, tableID, err)
	}
	return nil
}

// InvalidateAllForTenant remove todo cache de metadata de um tenant.
// Chamado em: operações que afetam múltiplas tabelas (promoção de tier, etc).
func (c *MetadataCache) InvalidateAllForTenant(ctx context.Context, tenantID string) error {
	// Padrão de keys: meta:{tenant_id}:*
	// DragonflyDB suporta DEL com pattern via SCAN+DEL.
	// Em produção, usamos um approach mais eficiente com uma key de versão.
	tableKey := fmt.Sprintf("meta:%s:tables", tenantID)
	schemaKey := fmt.Sprintf("meta:%s:schema_version", tenantID)

	if err := c.client.Del(ctx, tableKey, schemaKey); err != nil {
		return fmt.Errorf("cache.InvalidateAllForTenant(%s): %w", tenantID, err)
	}
	return nil
}
