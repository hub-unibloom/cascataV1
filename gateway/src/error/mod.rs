// Módulo de erros do Gateway.
// Ref: SAD §B (CascataValidationError), SRS Req-2.19.5, Req-2.19.7, Regra 3.2.
pub mod pg_translator;

use thiserror::Error;

use crate::validation::error::CascataValidationError;

/// Erro do Gateway — todo erro do YugabyteDB/pgcat é traduzido para este formato.
/// Erros do PostgreSQL NUNCA são expostos diretamente ao cliente (Regra 3.2).
#[derive(Debug, Error)]
pub enum GatewayError {
    /// Erro de validação estruturada — HTTP 422 (Req-2.19.5).
    /// Retornado quando o Validation Engine detecta violações no payload.
    #[error("validation failed")]
    ValidationFailed(CascataValidationError),

    /// Erro de constraint do banco traduzido — HTTP 422 (Req-2.19.7).
    /// Retornado quando o YugabyteDB rejeita mas o Pingora traduz para formato seguro.
    #[error("database constraint violation: {message}")]
    DatabaseConstraint {
        message: String,
        field: Option<String>,
    },

    /// Erro genérico de validação de input — HTTP 400.
    /// Para erros que não são validações de schema (ex: JSON malformado).
    #[error("bad request: {message}")]
    BadRequest { message: String },

    #[error("unauthorized: {0}")]
    Unauthorized(String),

    #[error("forbidden: {0}")]
    Forbidden(String),

    #[error("not found: {0}")]
    NotFound(String),

    #[error("rate limited: retry after {retry_after_ms}ms")]
    RateLimited { retry_after_ms: u64 },

    #[error("upstream error")]
    Upstream(#[from] anyhow::Error),

    #[error("internal error")]
    Internal(String),
}

impl GatewayError {
    /// Retorna o HTTP status code correspondente.
    pub fn status_code(&self) -> u16 {
        match self {
            Self::ValidationFailed(_) => 422,     // Unprocessable Entity (Req-2.19.5)
            Self::DatabaseConstraint { .. } => 422, // Constraint violation traduzida (Req-2.19.7)
            Self::BadRequest { .. } => 400,
            Self::Unauthorized(_) => 401,
            Self::Forbidden(_) => 403,
            Self::NotFound(_) => 404,
            Self::RateLimited { .. } => 429,
            Self::Upstream(_) | Self::Internal(_) => 500,
        }
    }

    /// Serializa o erro para resposta JSON ao cliente.
    /// Nunca expõe informação interna (Regra 3.2).
    pub fn to_json_body(&self) -> serde_json::Value {
        match self {
            Self::ValidationFailed(err) => serde_json::to_value(err)
                .unwrap_or_else(|_| serde_json::json!({"error": "validation_failed"})),
            Self::DatabaseConstraint { message, field } => {
                let mut violation = serde_json::json!({
                    "error": "validation_failed",
                    "violations": [{
                        "rule": "database_constraint",
                        "message": message,
                    }]
                });
                if let Some(f) = field {
                    violation["violations"][0]["field"] = serde_json::json!(f);
                }
                violation
            }
            Self::BadRequest { message } => {
                serde_json::json!({"error": "bad_request", "message": message})
            }
            Self::Unauthorized(_) => serde_json::json!({"error": "unauthorized"}),
            Self::Forbidden(_) => serde_json::json!({"error": "forbidden"}),
            Self::NotFound(_) => serde_json::json!({"error": "not_found"}),
            Self::RateLimited { retry_after_ms } => {
                serde_json::json!({"error": "rate_limited", "retry_after_ms": retry_after_ms})
            }
            // Erros internos NUNCA expõem detalhes ao cliente
            Self::Upstream(_) | Self::Internal(_) => {
                serde_json::json!({"error": "internal_error", "message": "Erro interno. Contate o suporte se o problema persistir."})
            }
        }
    }
}
