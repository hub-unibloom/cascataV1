package pool

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestRLSHandshakeLeak valida o requisito 0.1.16 da TASK.
// "Teste de validação: o RLS Handshake deve funcionar sem vazamento de claims entre transações"
func TestRLSHandshakeLeak(t *testing.T) {
	// Conecta nativamente via porta client do banco
	dsn := "postgres://cascata_pool_user:dummy@localhost:5433/cascata_shared"
	
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("Banco não acessível, skip teste de integração: %v", err)
	}
	defer db.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Pool (YSQL CM) indisponível localmente para teste: %v", err)
	}

	// Request 1: Seta as 3 claims na transação e faz commit
	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Erro abrindo tx1: %v", err)
	}
	_, err = tx1.ExecContext(ctx, `
		SELECT set_config('request.jwt.claim.sub', 'user-123', false);
		SELECT set_config('request.jwt.claim.tenant_id', 'tenant-123', false);
		SELECT set_config('request.jwt.claim.role', 'admin', false);
	`)
	if err != nil {
		t.Fatalf("Erro set_config em tx1: %v", err)
	}
	// O COMMIT devolve a conexão pro pool. 
	// A server_reset_query deve zerar as 3 claims.
	if err := tx1.Commit(); err != nil {
		t.Fatalf("Erro commit tx1: %v", err)
	}

	// Request 2: Inicia nova transação (pegará a mesma conexão se não houver concorrência no teste)
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Erro abrindo tx2: %v", err)
	}
	defer tx2.Rollback()

	var sub, tenantID, role sql.NullString
	err = tx2.QueryRowContext(ctx, `
		SELECT 
			current_setting('request.jwt.claim.sub', true),
			current_setting('request.jwt.claim.tenant_id', true),
			current_setting('request.jwt.claim.role', true)
	`).Scan(&sub, &tenantID, &role)
	if err != nil {
		t.Fatalf("Erro lendo claims em tx2: %v", err)
	}

	if (sub.Valid && sub.String != "") || 
	   (tenantID.Valid && tenantID.String != "") || 
	   (role.Valid && role.String != "") {
		t.Errorf("VAZAMENTO CRÍTICO DETECTADO! Conexão reteve o state anterior: sub=%s, tenant_id=%s, role=%s", sub.String, tenantID.String, role.String)
	} else {
		t.Log("Sucesso: Nenhuma claim (sub, tenant_id, role) persistiu. O RLS Handshake funciona blindado.")
	}
}
