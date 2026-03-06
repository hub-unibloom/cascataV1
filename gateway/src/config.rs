// Configuração do gateway Pingora.
// Carregada de arquivo TOML + override por env vars.
// Zero unwrap() — todo erro propagado com ? (Regra 5.2).
// Ref: SAD §B, TASK PR-6.

use anyhow::{Context, Result};
use serde::Deserialize;
use std::path::Path;

/// Configuração raiz do Gateway.
#[derive(Debug, Clone, Deserialize)]
pub struct GatewayConfig {
    /// Endereço de escuta HTTP (ex: "0.0.0.0:8080")
    #[serde(default = "default_listen_addr")]
    pub listen_addr: String,

    /// Endereço de escuta HTTPS (ex: "0.0.0.0:8443")
    #[serde(default = "default_tls_addr")]
    pub tls_addr: String,

    /// Endereço de health check (ex: "0.0.0.0:8081")
    #[serde(default = "default_health_addr")]
    pub health_addr: String,

    /// Upstream (pgcat)
    #[serde(default)]
    pub upstream: UpstreamConfig,

    /// TLS
    #[serde(default)]
    pub tls: TlsConfig,
}

/// Configuração do upstream (pgcat connection pooler).
#[derive(Debug, Clone, Deserialize)]
pub struct UpstreamConfig {
    /// Host do pgcat (nome do service no Docker, ex: "pgcat")
    #[serde(default = "default_upstream_host")]
    pub host: String,

    /// Porta do YugabyteDB (5433)
    #[serde(default = "default_upstream_port")]
    pub port: u16,
}

/// Configuração TLS.
#[derive(Debug, Clone, Deserialize)]
pub struct TlsConfig {
    /// Habilitar TLS
    #[serde(default)]
    pub enabled: bool,

    /// Path do certificado
    #[serde(default)]
    pub cert_path: String,

    /// Path da chave privada
    #[serde(default)]
    pub key_path: String,
}

// Defaults — valores para Shelter mode / dev
fn default_listen_addr() -> String { "0.0.0.0:8080".to_string() }
fn default_tls_addr() -> String { "0.0.0.0:8443".to_string() }
fn default_health_addr() -> String { "0.0.0.0:8081".to_string() }
fn default_upstream_host() -> String { "yugabytedb".to_string() }
fn default_upstream_port() -> u16 { 5433 }

impl Default for UpstreamConfig {
    fn default() -> Self {
        Self {
            host: default_upstream_host(),
            port: default_upstream_port(),
        }
    }
}

impl Default for TlsConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            cert_path: String::new(),
            key_path: String::new(),
        }
    }
}

impl GatewayConfig {
    /// Carrega configuração de um arquivo TOML.
    /// Path: env `CASCATA_GATEWAY_CONFIG`, default "/etc/cascata/gateway.toml".
    pub fn load() -> Result<Self> {
        let config_path = std::env::var("CASCATA_GATEWAY_CONFIG")
            .unwrap_or_else(|_| "/etc/cascata/gateway.toml".to_string());

        let path = Path::new(&config_path);

        if path.exists() {
            let content = std::fs::read_to_string(path)
                .with_context(|| format!("reading config file: {}", config_path))?;

            let cfg: GatewayConfig = toml::from_str(&content)
                .with_context(|| format!("parsing config file: {}", config_path))?;

            Ok(cfg)
        } else {
            // Sem arquivo de config — usar defaults puros
            tracing::warn!("config file not found at {}, using defaults", config_path);
            Ok(Self::default())
        }
    }

    /// Retorna o endereço upstream completo (host:port).
    pub fn upstream_addr(&self) -> String {
        format!("{}:{}", self.upstream.host, self.upstream.port)
    }
}

impl Default for GatewayConfig {
    fn default() -> Self {
        Self {
            listen_addr: default_listen_addr(),
            tls_addr: default_tls_addr(),
            health_addr: default_health_addr(),
            upstream: UpstreamConfig::default(),
            tls: TlsConfig::default(),
        }
    }
}
