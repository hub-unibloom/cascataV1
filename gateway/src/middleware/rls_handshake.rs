// RLS Handshake — setup de Row Level Security no upstream.
// Injeta SET LOCAL ROLE e set_config claims antes da query.
// Ref: SAD §B (RLS Handshake atômico), SRS Req-2.2.
// Implementação: task 0.6.

/// Parâmetros do RLS Handshake a injetar no upstream.
#[derive(Debug, Clone)]
pub struct RlsParams {
    pub db_role: String,     // "cascata_api_role"
    pub user_id: String,     // JWT claim.sub
    pub tenant_id: String,   // Identificador do tenant
    pub user_role: String,   // JWT claim.role
}

/// Gera os comandos SQL do RLS Handshake.
/// Execute in order: BEGIN; SET LOCAL ROLE; set_config...
pub fn build_handshake_sql(params: &RlsParams) -> Vec<String> {
    vec![
        "BEGIN".to_string(),
        format!("SET LOCAL ROLE {}", params.db_role),
        format!("SELECT set_config('request.jwt.claim.sub', '{}', true)", params.user_id),
        format!("SELECT set_config('request.jwt.claim.tenant_id', '{}', true)", params.tenant_id),
        format!("SELECT set_config('request.jwt.claim.role', '{}', true)", params.user_role),
    ]
}
