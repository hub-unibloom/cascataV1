// Upstream resolution — resolve qual backend recebe o request.
// Shelter mode: single upstream (pgcat). Produção: Tenant Router.
// Ref: SAD §3 (Caminho A, step 3), TASK PR-6.

/// Representa um upstream backend resolvido pelo Tenant Router.
/// No skeleton, sempre resolve para pgcat. Quando o Tenant Router
/// estiver implementado (task 0.7), este módulo resolverá por tier.
#[derive(Debug, Clone)]
pub struct UpstreamTarget {
    pub host: String,
    pub port: u16,
    pub tls: bool,
    pub sni: String,
}

impl UpstreamTarget {
    /// Cria um target padrão para o YSQL CM local.
    pub fn ysql_cm_default() -> Self {
        Self {
            host: "yugabytedb".to_string(),
            port: 5433,
            tls: false,
            sni: String::new(),
        }
    }

    /// Retorna o endereço como tuple (host, port).
    pub fn addr(&self) -> (&str, u16) {
        (&self.host, self.port)
    }
}
