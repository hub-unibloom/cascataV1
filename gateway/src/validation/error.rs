// Validation errors — erros de validação formatados para o cliente.
// Mensagens legíveis, contexto de campo, sem informação interna exposta.
// Ref: SRS Req-2.19.5 (Resposta de Erro Estruturada), Regra 3.2.
// Implementação: task 0.6.

use serde::Serialize;

/// Resposta de erro de validação para o cliente (HTTP 422).
/// Formato: { "error": "validation_failed", "violations": [...] }
/// SDK tipifica como CascataValidationError (Req-2.19.5).
#[derive(Debug, Serialize)]
pub struct CascataValidationError {
    pub error: String,      // Sempre "validation_failed"
    pub violations: Vec<FieldViolation>,
}

impl CascataValidationError {
    pub fn new(violations: Vec<FieldViolation>) -> Self {
        Self {
            error: "validation_failed".to_string(),
            violations,
        }
    }
}

/// Violação individual por campo (Req-2.19.5).
#[derive(Debug, Serialize)]
pub struct FieldViolation {
    pub field: String,
    pub rule: String,           // "required", "regex", "range", etc.
    pub message: String,        // Mensagem legível configurada pelo tenant
    #[serde(skip_serializing_if = "Option::is_none")]
    pub value_received: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub expression: Option<String>,  // Para cross_field e jwt_context
}
