// JWT validation middleware.
// Valida Bearer token em todo request autenticado.
// Ref: SAD §B (Auth Engine), SRS Req-2.2.
// Implementação: task 0.6.

/// Claims extraídos do JWT após validação.
#[derive(Debug, Clone)]
pub struct JwtClaims {
    pub sub: String,            // Subject (user ID)
    pub tenant_id: String,      // Tenant ao qual o user pertence
    pub role: String,           // Role do usuário
    pub exp: u64,               // Expiração (Unix timestamp)
}

/// Valida um JWT e retorna os claims.
/// Stub — retorna erro até ser implementado na task 0.6.
pub fn validate_jwt(_token: &str) -> Result<JwtClaims, crate::error::GatewayError> {
    Err(crate::error::GatewayError::Internal(
        "JWT validation not implemented yet (task 0.6)".to_string(),
    ))
}
