// Package extensions — Implementação da Tipagem Nativa de Erros Inbound para Interface Web (Cascata V1)
// Ref: SRS Req-2.19.5 (Formato estruturado de Erros via SDK)
// Ref: TASK_Fase0_Alicerce_Invisivel 0.6.10
package extensions

import (
	"fmt"
)

// CascataValidationError define o envelope genérico de erro mapeado que flui para as bordas.
// Ele é responsável por transportar o error state validado (Zero DB Leakage).
type CascataValidationError struct {
	HTTPStatus int
	Code       string
	Message    string
	Extension  string
	TenantID   string
	Operation  string
}

// Error implementa a interface nativa golang/error de forma coesa.
func (e *CascataValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ToHTTP formata a saída em envelope estruturado para payload JSON/SDK (Resposta 4xx e 5xx).
// Garante o compliance do Req-2.19.5 para o Front-end/Dashboard.
func (e *CascataValidationError) ToHTTP() map[string]interface{} {
	return map[string]interface{}{
		"error":   e.Code,
		"message": e.Message,
		"details": map[string]interface{}{
			"extension": e.Extension,
			"tenant_id": e.TenantID,
			"operation": e.Operation,
		},
	}
}

// ExtensionDependency representa um objeto isolado do banco de dados dependente de recursos em uma extensão,
// vital para prover diagnósticos claros e concisos onde DROPs diretos falhariam sem CASCADE explícito.
type ExtensionDependency struct {
	ObjectType string `json:"object_type"` // Ex: 'table', 'function', 'type', 'view'
	ObjectName string `json:"object_name"` // Nome qualificado da procedure ou constraint ativada
	DepType    string `json:"dep_type"`    // Ex:'n' = normal dependency, 'a' = auto-created dependency
}

// CascataExtensionDependencyError transporta informações cirúrgicas extraídas dos DDLs do banco (pg_depend) 
// envelopadas na topologia de um erro padronizado para Modal e Alert rendering.
type CascataExtensionDependencyError struct {
	Extension    string
	Dependencies []ExtensionDependency
	Message      string
}

// Error implementa a interface error capturada pela pipeline gin/net/http golang.
func (e *CascataExtensionDependencyError) Error() string {
	return e.Message
}

// ToHTTP empacota a notificação nativa injetando o array list (violations) que o painel usa para iterar componentes dependentes.
// A interface compartilha nativamente o layout JSON do CascataValidationError e Engine de Validação (0.6.4).
func (e *CascataExtensionDependencyError) ToHTTP() map[string]interface{} {
	return map[string]interface{}{
		"error":      "extension_has_dependents",
		"message":    e.Message,
		"violations": buildViolationsFromDeps(e.Dependencies),
	}
}

// buildViolationsFromDeps é um utilitário de transição (translator).
// Ele adapta o formato bruto extraído de `pg_depend` e `pg_class` em um design limpo (JSON compliance violation struct).
func buildViolationsFromDeps(deps []ExtensionDependency) []map[string]interface{} {
	var violations []map[string]interface{}
	
	for _, dep := range deps {
		violations = append(violations, map[string]interface{}{
			"field":   dep.ObjectName,
			"rule":    dep.ObjectType,
			"message": fmt.Sprintf("Objeto dependente do tipo %s: %s", dep.ObjectType, dep.ObjectName),
		})
	}
	
	// Retornamos arrays zerados e não nulls, provendo estabilidade e compatibilidade rígida com Parsers JS.
	if violations == nil {
		return []map[string]interface{}{}
	}
	
	return violations
}
