// Rate Limit middleware consultando DragonflyDB.
// Estado de consumo por chave/grupo/usuário no DragonflyDB (<0.1ms lookup).
// Ref: SAD §B (Rate Limit Nerf), SRS Req-2.15.
// Implementação: task 0.1.

/// Resultado de verificação de rate limit.
#[derive(Debug)]
pub enum RateLimitResult {
    /// Request permitido.
    Allowed { remaining: u64 },
    /// Limite atingido — request negado ou nerfado.
    Limited { retry_after_ms: u64 },
}

/// Verifica rate limit para uma chave.
/// Stub — permite tudo até ser implementado na task 0.1.
pub fn check_rate_limit(_key: &str) -> RateLimitResult {
    // TODO: Consultar DragonflyDB na task 0.1
    RateLimitResult::Allowed { remaining: u64::MAX }
}
