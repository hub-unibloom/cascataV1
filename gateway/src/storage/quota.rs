// Quota reservation — reserva preventiva de cota no DragonflyDB.
// Feita ANTES de aceitar o upload (SAD §B Storage Router).
// Ref: SAD §B, SRS Req-2.6.

/// Resultado de verificação de cota.
#[derive(Debug)]
pub enum QuotaResult {
    Allowed { remaining_bytes: u64 },
    Exceeded { limit_bytes: u64, used_bytes: u64 },
}

/// Verifica e reserva cota para um upload.
/// Stub — permite tudo até ser implementado.
pub fn check_and_reserve(_tenant_id: &str, _file_size: u64) -> QuotaResult {
    // TODO: Consultar DragonflyDB e reservar
    QuotaResult::Allowed { remaining_bytes: u64::MAX }
}
