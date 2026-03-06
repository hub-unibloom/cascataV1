// Módulo proxy — implementação do ProxyHttp trait para o Pingora.
// CascataProxy é o coração do Data Plane: recebe requests HTTP e
// as encaminha para o upstream (pgcat → YugabyteDB).
// Ref: SAD §B, §3 (Caminho A), TASK PR-6.

pub mod upstream;

use async_trait::async_trait;
use pingora_core::upstreams::peer::HttpPeer;
use pingora_core::Result;
use pingora_http::RequestHeader;
use pingora_proxy::{ProxyHttp, Session};
use pingora_http::ResponseHeader;

use crate::config::GatewayConfig;
use crate::error::GatewayError;

/// CascataProxy implementa o ProxyHttp trait do Pingora.
/// Pipeline executado a cada request na seguinte ordem (SAD §B):
///   1. JWT validation + Cedar ABAC (middleware/jwt.rs, middleware/abac.rs)
///   2. Rate Limit consultando DragonflyDB (middleware/rate_limit.rs)
///   3. RLS Handshake setup (middleware/rls_handshake.rs)
///   4. Validation Engine (validation/engine.rs)
///   5. Computed Columns (computed/api_columns.rs)
///   6. Proxy pass → YugabyteDB:5433
///
/// Neste skeleton (PR-6): apenas o proxy pass funciona.
/// Os middleware serão implementados nas tasks 0.x correspondentes.
pub struct CascataProxy {
    upstream_host: String,
    upstream_port: u16,
}

impl CascataProxy {
    /// Cria um novo CascataProxy a partir da configuração.
    pub fn new(cfg: &GatewayConfig) -> Self {
        Self {
            upstream_host: cfg.upstream.host.clone(),
            upstream_port: cfg.upstream.port,
        }
    }
}

#[async_trait]
impl ProxyHttp for CascataProxy {
    type CTX = ();

    fn new_ctx(&self) -> Self::CTX {}

    /// request_filter — executa antes do proxy forward.
    /// Capturamos o body (caso POST/PUT/PATCH) para aplicar a validação do schema.
    async fn request_filter(&self, session: &mut Session, _ctx: &mut Self::CTX) -> Result<bool> {
        let req = session.req_header();
        
        // Apenas intercepta operações verbosas de mutação
        if req.method == http::Method::POST || req.method == http::Method::PUT || req.method == http::Method::PATCH {
            // Bufereza o payload
            if let Ok(Some(body_bytes)) = session.read_request_body().await {
                if let Ok(json_body) = serde_json::from_slice::<serde_json::Value>(&body_bytes) {
                    
                    // Stub JWT
                    let dummy_jwt = serde_json::json!({"sub": "anonymous"});
                    
                    // TODO (0.6): Atingir o DragonflyDB e compilar rules a partir de FlatValidationRule
                    let rules: Vec<crate::validation::engine::ValidationRule> = vec![];
                    
                    let result = crate::validation::engine::validate(&rules, &json_body, &dummy_jwt);
                    
                    if !result.valid {
                        let err = GatewayError::ValidationFailed(result.to_error_response());
                        let resp_body = err.to_json_body().to_string();
                        
                        let mut header = ResponseHeader::build(err.status_code(), None).unwrap();
                        header.insert_header("Content-Type", "application/json").unwrap();
                        
                        session.write_response_header(Box::new(header), false).await?;
                        session.write_response_body(resp_body.into(), true).await?;
                        
                        return Ok(true); // Bypassa o upstream
                    }
                }
            }
        }
        
        Ok(false)
    }

    /// upstream_peer — resolve para qual backend enviar o request.
    /// No Shelter mode: sempre pgcat (single upstream).
    /// Em produção: Tenant Router resolve por tier.
    async fn upstream_peer(
        &self,
        _session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> Result<Box<HttpPeer>> {
        let peer = HttpPeer::new(
            (&self.upstream_host as &str, self.upstream_port),
            false, // TLS ao upstream: false no Shelter
            String::new(),
        );
        Ok(Box::new(peer))
    }

    /// upstream_request_filter — modifica o request antes de enviar ao upstream.
    /// Aqui serão adicionados headers RLS, JWT claims, etc.
    /// Skeleton: passa o request sem modificação.
    async fn upstream_request_filter(
        &self,
        _session: &mut Session,
        _upstream_request: &mut RequestHeader,
        _ctx: &mut Self::CTX,
    ) -> Result<()> {
        // TODO: Task 0.6 — Injetar headers RLS (SET LOCAL ROLE, set_config claims)
        // TODO: Task 0.1 — Rate Limit check via DragonflyDB
        // TODO: Task 0.5 — Validation Engine execution
        Ok(())
    }
}
