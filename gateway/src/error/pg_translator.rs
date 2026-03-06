// Tradutor de erros PostgreSQL/YugabyteDB → CascataValidationError.
// Erros do banco NUNCA são expostos diretamente ao cliente (Regra 3.2, SRS Req-2.19.7).
// O tenant nunca vê mensagens de erro cruas do PostgreSQL — em nenhuma circunstância.
//
// Mapeia SQLSTATE codes para mensagens legíveis e seguras, extraindo
// o nome do campo do `detail` da mensagem PG quando disponível.
//
// Ref: SRS Req-2.19.7, SAD §B, Regra 3.2.

use crate::validation::error::{CascataValidationError, FieldViolation};

/// Informação de erro do PostgreSQL/YugabyteDB recebida pelo gateway.
#[derive(Debug)]
pub struct PgErrorInfo {
    /// Código SQLSTATE de 5 caracteres (ex: "23505").
    pub sqlstate: String,
    /// Mensagem principal do erro PG.
    pub message: String,
    /// Campo `detail` do erro PG (opcional) — contém informação sobre o campo/valor.
    pub detail: Option<String>,
    /// Nome da constraint violada (opcional).
    pub constraint_name: Option<String>,
    /// Nome da tabela envolvida (opcional).
    pub table_name: Option<String>,
    /// Nome da coluna envolvida (opcional).
    pub column_name: Option<String>,
}

/// Traduz um erro do PostgreSQL/YugabyteDB para CascataValidationError.
/// A mensagem original do PG é consumida para contexto mas NUNCA retornada ao cliente.
/// Ref: SRS Req-2.19.7.
pub fn translate_pg_error(info: &PgErrorInfo) -> CascataValidationError {
    let violation = match info.sqlstate.as_str() {
        // === Integridade de dados ===

        // 23505 — unique_violation
        // "duplicate key value violates unique constraint"
        "23505" => {
            let field = extract_field_from_constraint(&info.constraint_name, &info.column_name);
            FieldViolation {
                field: field.unwrap_or_else(|| "_unknown".to_string()),
                rule: "unique".to_string(),
                message: format!(
                    "Registro duplicado. {}",
                    field
                        .as_ref()
                        .map(|f| format!("O campo '{}' já existe com este valor.", f))
                        .unwrap_or_else(|| "Um registro com estes dados já existe.".to_string())
                ),
                value_received: extract_value_from_detail(&info.detail),
                expression: None,
            }
        }

        // 23503 — foreign_key_violation
        // "insert or update on table violates foreign key constraint"
        "23503" => {
            let field = extract_field_from_constraint(&info.constraint_name, &info.column_name);
            FieldViolation {
                field: field.unwrap_or_else(|| "_unknown".to_string()),
                rule: "foreign_key".to_string(),
                message: "Referência inválida. O registro relacionado não existe.".to_string(),
                value_received: extract_value_from_detail(&info.detail),
                expression: None,
            }
        }

        // 23514 — check_violation
        // "new row for relation violates check constraint"
        "23514" => {
            let field = info.column_name.clone();
            FieldViolation {
                field: field.unwrap_or_else(|| "_unknown".to_string()),
                rule: "check_constraint".to_string(),
                message: format!(
                    "Valor viola restrição de integridade{}.",
                    info.column_name
                        .as_ref()
                        .map(|f| format!(" no campo '{}'", f))
                        .unwrap_or_default()
                ),
                value_received: None,
                expression: None,
            }
        }

        // 23502 — not_null_violation
        // "null value in column violates not-null constraint"
        "23502" => {
            let field = info
                .column_name
                .clone()
                .unwrap_or_else(|| "_unknown".to_string());
            FieldViolation {
                field: field.clone(),
                rule: "required".to_string(),
                message: format!("Campo obrigatório '{}' não pode ser vazio.", field),
                value_received: Some("null".to_string()),
                expression: None,
            }
        }

        // === Tipos e formatos ===

        // 22P02 — invalid_text_representation
        // "invalid input syntax for type..."
        "22P02" => {
            let field = info.column_name.clone();
            FieldViolation {
                field: field.unwrap_or_else(|| "_unknown".to_string()),
                rule: "type".to_string(),
                message: format!(
                    "Formato de dado inválido{}.",
                    info.column_name
                        .as_ref()
                        .map(|f| format!(" para o campo '{}'", f))
                        .unwrap_or_default()
                ),
                value_received: None,
                expression: None,
            }
        }

        // 22003 — numeric_value_out_of_range
        "22003" => FieldViolation {
            field: info
                .column_name
                .clone()
                .unwrap_or_else(|| "_unknown".to_string()),
            rule: "range".to_string(),
            message: "Valor numérico fora do intervalo permitido.".to_string(),
            value_received: None,
            expression: None,
        },

        // 22001 — string_data_right_truncation
        "22001" => FieldViolation {
            field: info
                .column_name
                .clone()
                .unwrap_or_else(|| "_unknown".to_string()),
            rule: "length".to_string(),
            message: "Texto excede o tamanho máximo permitido.".to_string(),
            value_received: None,
            expression: None,
        },

        // === Referência a objetos ===

        // 42P01 — undefined_table
        "42P01" => FieldViolation {
            field: "_table".to_string(),
            rule: "not_found".to_string(),
            message: "Recurso não encontrado.".to_string(),
            value_received: None,
            expression: None,
        },

        // 42703 — undefined_column
        "42703" => {
            let field = extract_column_from_message(&info.message);
            FieldViolation {
                field: field.unwrap_or_else(|| "_unknown".to_string()),
                rule: "not_found".to_string(),
                message: format!(
                    "Campo '{}' não existe neste recurso.",
                    field.as_deref().unwrap_or("desconhecido")
                ),
                value_received: None,
                expression: None,
            }
        }

        // === Exclusão restrita ===

        // 23503 com mensagem de delete → restrict violation
        // (Detectado pelo caller via contexto da operação)

        // === Fallback para todos os outros SQLSTATE ===
        _ => FieldViolation {
            field: "_database".to_string(),
            rule: "database_error".to_string(),
            message: "Erro de banco de dados. Contate o suporte se o problema persistir."
                .to_string(),
            value_received: None,
            expression: None,
        },
    };

    CascataValidationError::new(vec![violation])
}

/// Extrai o nome do campo a partir do nome da constraint ou da coluna PG.
///
/// Constraints do Cascata seguem o pattern:
///   `{table}_{column}_key` (unique)
///   `{table}_{column}_fkey` (foreign key)
///   `{table}_{column}_check` (check)
fn extract_field_from_constraint(
    constraint_name: &Option<String>,
    column_name: &Option<String>,
) -> Option<String> {
    // Prioridade: coluna explícita > extração da constraint
    if let Some(col) = column_name {
        if !col.is_empty() {
            return Some(col.clone());
        }
    }

    // Tentar extrair do nome da constraint
    if let Some(name) = constraint_name {
        // Padrão: table_column_key, table_column_fkey, table_column_check
        let parts: Vec<&str> = name.rsplitn(2, '_').collect();
        if parts.len() == 2 {
            let suffix = parts[0];
            if suffix == "key" || suffix == "fkey" || suffix == "check" || suffix == "pkey" {
                // O restante pode conter table_column — pegar última parte antes do sufixo
                let remaining = parts[1];
                if let Some(col) = remaining.rsplit('_').next() {
                    return Some(col.to_string());
                }
            }
        }
    }

    None
}

/// Tenta extrair o valor do `detail` da mensagem PG.
///
/// Pattern típico no detail:
///   "Key (email)=(user@test.com) already exists."
///   "Key (id)=(uuid) is not present in table..."
fn extract_value_from_detail(detail: &Option<String>) -> Option<String> {
    let d = detail.as_ref()?;
    // Procurar pattern =(valor)
    let start = d.find("=(")? + 2;
    let end = d[start..].find(')')? + start;
    Some(d[start..end].to_string())
}

/// Tenta extrair nome de coluna da mensagem de erro PG.
///
/// Pattern: 'column "colname" of relation...'
fn extract_column_from_message(message: &str) -> Option<String> {
    // Pattern: column "X" ou "X"  
    let start = message.find('"')? + 1;
    let end = message[start..].find('"')? + start;
    Some(message[start..end].to_string())
}

// ===========================================================================
// TESTES
// ===========================================================================

#[cfg(test)]
mod tests {
    use super::*;

    fn make_pg_error(sqlstate: &str, message: &str) -> PgErrorInfo {
        PgErrorInfo {
            sqlstate: sqlstate.to_string(),
            message: message.to_string(),
            detail: None,
            constraint_name: None,
            table_name: None,
            column_name: None,
        }
    }

    #[test]
    fn test_unique_violation_basic() {
        let info = PgErrorInfo {
            sqlstate: "23505".to_string(),
            message: "duplicate key value violates unique constraint".to_string(),
            detail: Some("Key (email)=(user@test.com) already exists.".to_string()),
            constraint_name: Some("users_email_key".to_string()),
            table_name: Some("users".to_string()),
            column_name: Some("email".to_string()),
        };
        let err = translate_pg_error(&info);
        assert_eq!(err.error, "validation_failed");
        assert_eq!(err.violations.len(), 1);
        assert_eq!(err.violations[0].field, "email");
        assert_eq!(err.violations[0].rule, "unique");
        assert_eq!(
            err.violations[0].value_received.as_deref(),
            Some("user@test.com")
        );
        assert!(err.violations[0].message.contains("Registro duplicado"));
    }

    #[test]
    fn test_foreign_key_violation() {
        let info = PgErrorInfo {
            sqlstate: "23503".to_string(),
            message: "violates foreign key constraint".to_string(),
            detail: Some("Key (category_id)=(uuid) is not present in table \"categories\".".to_string()),
            constraint_name: Some("products_category_id_fkey".to_string()),
            table_name: Some("products".to_string()),
            column_name: Some("category_id".to_string()),
        };
        let err = translate_pg_error(&info);
        assert_eq!(err.violations[0].field, "category_id");
        assert_eq!(err.violations[0].rule, "foreign_key");
        assert!(err.violations[0].message.contains("Referência inválida"));
    }

    #[test]
    fn test_not_null_violation() {
        let info = PgErrorInfo {
            sqlstate: "23502".to_string(),
            message: "null value in column \"name\" violates not-null constraint".to_string(),
            detail: None,
            constraint_name: None,
            table_name: Some("users".to_string()),
            column_name: Some("name".to_string()),
        };
        let err = translate_pg_error(&info);
        assert_eq!(err.violations[0].field, "name");
        assert_eq!(err.violations[0].rule, "required");
        assert!(err.violations[0].message.contains("Campo obrigatório"));
    }

    #[test]
    fn test_check_violation() {
        let info = PgErrorInfo {
            sqlstate: "23514".to_string(),
            message: "new row for relation violates check constraint".to_string(),
            detail: None,
            constraint_name: Some("products_price_check".to_string()),
            table_name: Some("products".to_string()),
            column_name: Some("price".to_string()),
        };
        let err = translate_pg_error(&info);
        assert_eq!(err.violations[0].field, "price");
        assert_eq!(err.violations[0].rule, "check_constraint");
    }

    #[test]
    fn test_invalid_text_representation() {
        let err = translate_pg_error(&make_pg_error(
            "22P02",
            "invalid input syntax for type integer",
        ));
        assert_eq!(err.violations[0].rule, "type");
        assert!(err.violations[0].message.contains("Formato de dado inválido"));
    }

    #[test]
    fn test_numeric_out_of_range() {
        let err = translate_pg_error(&make_pg_error("22003", "numeric value out of range"));
        assert_eq!(err.violations[0].rule, "range");
    }

    #[test]
    fn test_string_truncation() {
        let err = translate_pg_error(&make_pg_error(
            "22001",
            "value too long for type character varying(100)",
        ));
        assert_eq!(err.violations[0].rule, "length");
    }

    #[test]
    fn test_undefined_table() {
        let err = translate_pg_error(&make_pg_error("42P01", "relation \"nonexistent\" does not exist"));
        assert_eq!(err.violations[0].rule, "not_found");
        assert_eq!(err.violations[0].field, "_table");
    }

    #[test]
    fn test_undefined_column() {
        let info = PgErrorInfo {
            sqlstate: "42703".to_string(),
            message: "column \"nonexistent\" of relation \"users\" does not exist".to_string(),
            detail: None,
            constraint_name: None,
            table_name: None,
            column_name: None,
        };
        let err = translate_pg_error(&info);
        assert_eq!(err.violations[0].rule, "not_found");
        assert_eq!(err.violations[0].field, "nonexistent");
    }

    #[test]
    fn test_unknown_sqlstate_fallback() {
        let err = translate_pg_error(&make_pg_error("99999", "some unknown error"));
        assert_eq!(err.violations[0].rule, "database_error");
        assert!(err.violations[0].message.contains("Contate o suporte"));
        // Mensagem original NÃO exposta
        assert!(!err.violations[0].message.contains("some unknown error"));
    }

    #[test]
    fn test_extract_value_from_detail() {
        let detail = Some("Key (email)=(test@example.com) already exists.".to_string());
        assert_eq!(
            extract_value_from_detail(&detail),
            Some("test@example.com".to_string())
        );
    }

    #[test]
    fn test_extract_column_from_message() {
        assert_eq!(
            extract_column_from_message("column \"name\" of relation \"users\" does not exist"),
            Some("name".to_string())
        );
    }

    #[test]
    fn test_never_exposes_raw_pg_message() {
        // Verificar que NENHUM SQLSTATE expõe a mensagem original do PG
        let sqlstates = [
            "23505", "23503", "23514", "23502", "22P02", "22003", "22001", "42P01", "42703",
            "99999",
        ];
        for code in &sqlstates {
            let info = PgErrorInfo {
                sqlstate: code.to_string(),
                message: "INTERNAL_PG_MESSAGE_SHOULD_NEVER_APPEAR".to_string(),
                detail: None,
                constraint_name: None,
                table_name: None,
                column_name: None,
            };
            let err = translate_pg_error(&info);
            let json = serde_json::to_string(&err).expect("serialize");
            assert!(
                !json.contains("INTERNAL_PG_MESSAGE"),
                "SQLSTATE {} leaked raw PG message in response!",
                code
            );
        }
    }
}
