// Storage Router — roteia upload para o provider correto.
// Baseado nas regras de storage governance do tenant.
// Ref: SAD §B (Storage Router), SRS Req-2.6.

/// Provider de storage resolvido pelo router.
#[derive(Debug, Clone)]
pub enum StorageProvider {
    MinIO { bucket: String, prefix: String },
    // Futuros providers: S3, GCS, Azure Blob (tiers ENTERPRISE/SOVEREIGN)
}

/// Roteia um upload para o provider correto baseado em setor e tenant config.
/// Stub — retorna MinIO default até ser implementado.
pub fn route(
    _tenant_id: &str,
    _sector: &super::magic_bytes::FileSector,
) -> StorageProvider {
    // TODO: Consultar config do tenant para regras de storage governance
    StorageProvider::MinIO {
        bucket: "cascata-uploads".to_string(),
        prefix: "default/".to_string(),
    }
}
