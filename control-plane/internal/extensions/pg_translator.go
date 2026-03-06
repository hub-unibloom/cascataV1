// Package extensions — Translator de Erros do PostgreSQL para o Motor de Extensões
// Ref: SRS Req-2.19.7 (Tenant nunca vê mensagem crua em nenhuma circunstância)
// Ref: TASK_Fase0_Alicerce_Invisivel 0.6.9
package extensions

import (
	"fmt"
	"regexp"
	"strings"
)

// TranslationContext embute contexto ambiental e de Tenant para a auditoria de erro,
// gerando traceabilidade rica de operações sem poluir as APIs externas com log interno.
type TranslationContext struct {
	Operation string // "create_extension", "drop_extension"
	Extension string // "postgis", "pg_cron"
	TenantID  string // ID para mapeamento de log cross-referenciado
}

// TranslationRule define a assinatura da regra de mapeamento.
type TranslationRule struct {
	HTTPStatus int
	Code       string
	Message    string
}

// extensionErrorMap cataloging error states specifically expected across cluster operations.
var extensionErrorMap = map[string]TranslationRule{
	// CREATE EXTENSION falhou por permissão (Tenant Role com falta de privilégios superset)
	"permission denied to create extension": {
		HTTPStatus: 403,
		Code:       "extension_permission_denied",
		Message:    "Permissão insuficiente para habilitar a extensão \"%s\". Entre em contato com o suporte.",
	},
	// Extensão não existe na imagem do cluster (Ex: postgis no image :shared)
	"could not open extension control file": {
		HTTPStatus: 422,
		Code:       "extension_not_available",
		Message:    "A extensão \"%s\" não está disponível na imagem atual do cluster. Verifique o Extension Marketplace e o seu Tier.",
	},
	// Conflito de versão (Package version mismatch)
	"has no installation script": {
		HTTPStatus: 422,
		Code:       "extension_version_conflict",
		Message:    "Conflito de versão nativa ao instalar a extensão \"%s\". Por favor, reporte este problema ao suporte.",
	},
	// Dependência de cascata não declarada (Ex: required extension pg_trgm is not installed para habilitar outro modulo)
	"required extension": {
		HTTPStatus: 422,
		Code:       "extension_dependency_missing",
		Message:    "A extensão \"%s\" não pôde ser ativada porque requer outra extensão primária instalada.",
	},
	// DROP com objetos dependentes residuais
	"cannot drop extension": {
		HTTPStatus: 422,
		Code:       "extension_has_dependents",
		Message:    "A extensão possui objetos nativos que impedem a remoção. Verifique os impactos de dependência.",
	},
	// Extensão já existe no namespace do schema
	"already exists": {
		HTTPStatus: 409,
		Code:       "extension_already_exists",
		Message:    "A extensão \"%s\" já está ativa neste projeto.",
	},
}

// PGTranslator struct provê um endpoint Singleton de sanitização (Isolamento PG).
type PGTranslator struct{}

// NewPGTranslator fabrica uma nova abstração de parser de erros PostgreSQL para Controller Ext.
func NewPGTranslator() *PGTranslator {
	return &PGTranslator{}
}

// Translate intermedeia e decanta o RawError injetado e converte na tipagem de API pública limpa (CascataValidationError).
// Executa proteção absoluta Zero-Leakage (se o erro for DB interno, memory failure ou Connection Drop, nunca será printado para o Tenant).
func (t *PGTranslator) Translate(err error, ctx TranslationContext) *CascataValidationError {
	if err == nil {
		return nil
	}

	rawErr := err.Error()

	for signature, rule := range extensionErrorMap {
		if strings.Contains(rawErr, signature) {

			finalMsg := rule.Message

			// Tratamento inteligente caso haja indicação da dependência perdida no regex para mensagens fluídas
			// Ex: "ERROR: required extension "postgis" is not installed"
			if signature == "required extension" {
				reqMatched := extractRequiredExtension(rawErr)
				if reqMatched != "" {
					finalMsg = fmt.Sprintf("A extensão \"%s\" requer que \"%s\" esteja habilitada primeiro.", ctx.Extension, reqMatched)
				}
			} else if strings.Contains(finalMsg, "%s") {
				// Repassa a formatação injetando o NOME da extensão pedida e interceptada
				count := strings.Count(finalMsg, "%s")
				args := make([]interface{}, count)
				for i := range args {
					args[i] = ctx.Extension
				}
				finalMsg = fmt.Sprintf(rule.Message, args...)
			}

			return &CascataValidationError{
				HTTPStatus: rule.HTTPStatus,
				Code:       rule.Code,
				Message:    finalMsg,
				Extension:  ctx.Extension,
				TenantID:   ctx.TenantID,
				Operation:  ctx.Operation,
			}
		}
	}

	// FALLBACK ABSOLUTO TIER-1 (Zero SQL Leakage Garantido)
	// Sob nenhuma circunstância (incluindo pânico de node YugabyteDB) uma query SQL original vaza na rede.
	return &CascataValidationError{
		HTTPStatus: 500,
		Code:       "extension_internal_system_error",
		Message:    fmt.Sprintf("Houve um erro transacional invisível ao processar a operação da extensão \"%s\". A equipe de engenharia foi notificada através do Logging Imutável.", ctx.Extension),
		Extension:  ctx.Extension,
		TenantID:   ctx.TenantID,
		Operation:  ctx.Operation,
	}
}

// extractRequiredExtension recupera do log nativo do Postgres qual dependência está faltando.
// Extrai apenas o grupo de aspas contendo o namespace puro da library faltante.
func extractRequiredExtension(raw string) string {
	re := regexp.MustCompile(`required extension "([^"]+)"`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
