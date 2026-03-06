// Package extensions — Enabler Mechanism (0.8.3)
// Ref: SRS Req-2.20.3, Req-2.19.7 (zero mensagem crua do PostgreSQL)
package extensions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrExtensionNotFound                  = errors.New("a extensão especificada não existe no catálogo oficial")
	ErrExtensionBlocked                   = errors.New("a extensão encontra-se bloqueada")
	ErrExtensionRequiresDedicatedInstance = errors.New("a extensão selecionada requer um projeto em tier STANDARD ou superior (imagem :full)")
)

// ExtensionEvent struct simplificada do Redpanda Event Sourcing
type ExtensionEvent struct {
	TenantID      string
	Extension     string
	Action        string // "enabled", "disabled", "blocked", "failed", "inconsistency_warning"
	ExecutedBy    string
	Result        string
	FailureReason string
	ImageVariant  string
	TierAtTime    string
}

// EventPublisher interface para publicação de auditoria
type EventPublisher interface {
	Publish(ctx context.Context, e ExtensionEvent) error
}

// Logger interface para internal tracking
type Logger interface {
	Warn(msg string, args ...interface{})
}

// DefaultLogger preenche a abstração simples
type DefaultLogger struct{}
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] %s %v\n", msg, args)
}

// MockTenant interface provendo isolamento para testabilidade
type MockTenant struct {
	ID           string
	ImageVariant string
	Tier         string
	DBSchema     string
}

// Enabler coordena a criação da extensão e suas salvaguardas (Pending → Active).
type Enabler struct {
	repo         *ExtensionRepository
	pgTranslator *PGTranslator
	events       EventPublisher
	logger       Logger
	tenantDB     *pgxpool.Pool // Proxy Executor
}

// NewEnabler inicializa a pipeline rigorosa
func NewEnabler(repo *ExtensionRepository, translator *PGTranslator, events EventPublisher, logger Logger, tenantDB *pgxpool.Pool) *Enabler {
	return &Enabler{
		repo:         repo,
		pgTranslator: translator,
		events:       events,
		logger:       logger,
		tenantDB:     tenantDB,
	}
}

// Enable implementa o fluxo de Habilitação Segura Dupla.
func (e *Enabler) Enable(ctx context.Context, tenant MockTenant, extensionName, memberID string) error {
	// 1. Validações de catálogo (Bloqueios e Imagens)
	ext, err := e.repo.GetCatalogEntry(ctx, extensionName)
	if err != nil || ext == nil {
		return ErrExtensionNotFound
	}
	if ext.Category == 4 {
		return fmt.Errorf("%w: %s", ErrExtensionBlocked, ext.BlockedReason)
	}
	if tenant.ImageVariant == "shared" && !ext.AvailableInShared {
		return ErrExtensionRequiresDedicatedInstance
	}

	// 2. PRIMEIRA ESCRITA — registrar intenção no CP com status 'pending'
	// Se a rede cair e o node morre, o Reconciliador limpará a sujeira 
	// Ou consolidará amanhã de manhã. Zero Ambiguity.
	record := TenantExtension{
		TenantID:  tenant.ID,
		Extension: extensionName,
		EnabledBy: memberID,
		Status:    "pending",
	}

	record, err = e.repo.InsertTenantExtension(ctx, record)
	if err != nil {
		return err // CP Indisponível (Sem intenção -> fail fast, empty impact)
	}

	// 3. SEGUNDA ESCRITA — executar CREATE EXTENSION no banco do tenant (YugabyteDB alvo)
	// Timeout de 30s. Extensões pesadas (geo) podem compilar em YSQL boot primário.
	extCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sql := fmt.Sprintf(`CREATE EXTENSION IF NOT EXISTS "%s" SCHEMA "%s"`, extensionName, tenant.DBSchema)
	_, pgRawError := e.tenantDB.Exec(extCtx, sql)

	if pgRawError != nil {
		// Leak Protection (Tenant não vê driver postgres panic)
		cascataErr := e.pgTranslator.Translate(pgRawError, TranslationContext{
			Operation: "extension_enable",
			Extension: extensionName,
			TenantID:  tenant.ID,
		})

		// Reverter o pending (Desfaz a intenção de CP)
		_ = e.repo.DeleteTenantExtension(ctx, tenant.ID, record.ID)

		// Publicar evento de falha no Redpanda → ClickHouse
		_ = e.events.Publish(ctx, ExtensionEvent{
			TenantID:      tenant.ID,
			Extension:     extensionName,
			Action:        "enabled",
			ExecutedBy:    memberID,
			Result:        "failed",
			FailureReason: cascataErr.Message,
			ImageVariant:  tenant.ImageVariant,
			TierAtTime:    tenant.Tier,
		})

		return cascataErr
	}

	// 4. CONFIRMAÇÃO — promover pending -> active
	if err := e.repo.ConfirmTenantExtension(ctx, record.ID); err != nil {
		// CREATE EXTENSION foi OK no Tenant, mas ocorreu split-brain no CP local.
		e.logger.Warn("extension_confirmation_failed_reconciler_will_fix",
			"tenant_id", tenant.ID,
			"extension", extensionName,
			"record_id", record.ID,
			"error", err,
		)
		// A operação continua com sucesso porque ocorreu do lado do Yugabyte.
	}

	// 5. Cache Invalidation omitido para simplificação.

	// 6. Publicar evento final de sucesso (Audit)
	_ = e.events.Publish(ctx, ExtensionEvent{
		TenantID:     tenant.ID,
		Extension:    extensionName,
		Action:       "enabled",
		ExecutedBy:   memberID,
		Result:       "success",
		ImageVariant: tenant.ImageVariant,
		TierAtTime:   tenant.Tier,
	})

	return nil
}

// Disable desabilita a extensão garantido a verificação prévia de que não existem
// objetos no banco de dados (Views, Tipos, Tabelas) que entrarão em colapso silencioso.
func (e *Enabler) Disable(ctx context.Context, tenant MockTenant, extensionName, memberID string) error {
	// 1. Verificar objetos dependentes no banco do tenant (Zero-Silent-Cascade)
	query := `
		SELECT
			d.classid::regclass AS object_type,
			CASE d.classid::regclass::text
				WHEN 'pg_class' THEN c.relname
				WHEN 'pg_proc'  THEN p.proname
				ELSE d.objid::text
			END AS object_name,
			d.deptype
		FROM pg_depend d
		LEFT JOIN pg_class c ON d.classid = 'pg_class'::regclass AND d.objid = c.oid
		LEFT JOIN pg_proc p  ON d.classid = 'pg_proc'::regclass  AND d.objid = p.oid
		WHERE d.refclassid = 'pg_extension'::regclass
		  AND d.refobjid = (
			  SELECT oid FROM pg_extension WHERE extname = $1
		  )
		  AND d.deptype != 'e'  -- exclui objetos inerentes da própria library d extension
	`

	rows, err := e.tenantDB.Query(ctx, query, extensionName)
	if err != nil {
		return err
	}
	defer rows.Close()

	var deps []ExtensionDependency
	for rows.Next() {
		var dep ExtensionDependency
		if err := rows.Scan(&dep.ObjectType, &dep.ObjectName, &dep.DepType); err != nil {
			return err
		}
		deps = append(deps, dep)
	}

	// 2. Extensão possui âncoras. Impedir Deleção e Notificar (Exception Segura p/ Modal)
	if len(deps) > 0 {
		return &CascataExtensionDependencyError{
			Extension:    extensionName,
			Dependencies: deps,
			Message: fmt.Sprintf(
				"A extensão \"%s\" possui %d objeto(s) dependente(s) ativos. "+
					"Remova as dependências antes de solicitar a desabilitação total.",
				extensionName, len(deps)),
		}
	}

	// 3. Tratamento especial Customizado para extensões complexas
	if extensionName == "pg_cron" {
		// A remoção da extensão exige explodir cronicamente os triggers pendentes do wrapper.
		// Ex: e.removePgCronWrappers(ctx, tenant.ID)
	}

	// 4. Deleção Limpa e Definitiva (DROP EXTENSION CASCADE)
	dropSQL := fmt.Sprintf(`DROP EXTENSION IF EXISTS "%s" CASCADE`, extensionName)
	_, dropErr := e.tenantDB.Exec(ctx, dropSQL)

	if dropErr != nil {
		return e.pgTranslator.Translate(dropErr, TranslationContext{
			Operation: "extension_disable",
			Extension: extensionName,
			TenantID:  tenant.ID,
		})
	}

	// 5. Deleta do Controle (CP)
	if err := e.repo.DeleteTenantExtension(ctx, tenant.ID, extensionName); err != nil {
		return err
	}

	// 6. Invalidação de Cache Macro (Omitido)

	// 7. Auditoria Positiva
	_ = e.events.Publish(ctx, ExtensionEvent{
		TenantID:   tenant.ID,
		Extension:  extensionName,
		Action:     "disabled",
		ExecutedBy: memberID,
		Result:     "success",
	})

	return nil
}
