// Entrypoint do Cascata Gateway — Pingora Data Plane.
// Pipeline: request → middleware chain → proxy pass → pgcat → YugabyteDB.
// Zero unwrap() em paths de produção (Regra 5.2).
// Ref: SRS Req-2.1.5, SAD §B, TASK PR-6.

mod config;
mod error;
mod middleware;
mod proxy;
mod storage;
mod validation;
mod computed;

use anyhow::Result;
use pingora_core::server::Server;
use pingora_core::server::configuration::ServerConf;
use pingora_proxy::http_proxy_service;
use tracing_subscriber::EnvFilter;

use crate::config::GatewayConfig;
use crate::proxy::CascataProxy;

fn main() -> Result<()> {
    // Inicializar tracing (structured logging)
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| EnvFilter::new("info")),
        )
        .json()
        .init();

    tracing::info!("cascata-gateway starting");

    // Carregar configuração
    let cfg = GatewayConfig::load()?;
    tracing::info!(
        listen = %cfg.listen_addr,
        upstream = %cfg.upstream_addr(),
        "configuration loaded"
    );

    // Criar servidor Pingora
    let mut server = Server::new_with_opt_and_conf(
        None, // sem CLI opt
        ServerConf {
            threads: 1, // Shelter mode: 1 thread. Produção: auto-detect.
            ..Default::default()
        },
    );

    server.bootstrap();

    // Criar o proxy service
    let proxy = CascataProxy::new(&cfg);
    let mut proxy_service = http_proxy_service(&server.configuration, proxy);

    // Bind na porta de escuta
    proxy_service.add_tcp(&cfg.listen_addr);

    // Registrar o service no server
    server.add_service(proxy_service);

    tracing::info!(addr = %cfg.listen_addr, "gateway listening");

    // Bloqueia até shutdown
    server.run_forever();
}
