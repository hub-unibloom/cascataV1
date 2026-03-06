// Validation Engine — executa validações definidas no schema metadata.
// Carrega do cache DragonflyDB (invalidado em mudanças de schema).
// Ordem de execução (SAD §Validation Engine):
//   Etapa 1: required — campos obrigatórios presentes
//   Etapa 2: type — tipos corretos para os campos presentes
//   Etapa 3: regex, range, length, enum — validações por campo
//   Etapa 4: cross_field — expressões entre campos (avaliador Rust)
//   Etapa 5: jwt_context — validações com claims do JWT
//   Etapa 6: unique_soft — queries de unicidade ao YugabyteDB (apenas quando 1-5 passam)
//
// Falha em etapa 1 → retorno imediato (campo obrigatório ausente invalida todo o resto).
// Etapas 2-5 coletam todas as violações antes de retornar.
// Etapa 6 executa apenas se 1-5 passaram (evita query desnecessária ao banco).
//
// Ref: SRS Req-2.19.1 (Tipos de Validação), Req-2.19.5 (Resposta Estruturada),
//      SAD §Validation Engine, TASK 0.6.
// Zero unwrap() — Regra 5.2.

use regex::Regex;
use serde::Deserialize;
use std::collections::HashMap;
use std::sync::OnceLock;

use super::error::{CascataValidationError, FieldViolation};
use super::expression;

/// Cache estático de regex compilados.
/// A compilação de regex é O(n) no padrão — caro por request.
/// OnceLock + HashMap garante compilação única por padrão distinto.
static REGEX_CACHE: OnceLock<std::sync::Mutex<HashMap<String, Result<Regex, String>>>> =
    OnceLock::new();

fn get_regex_cache() -> &'static std::sync::Mutex<HashMap<String, Result<Regex, String>>> {
    REGEX_CACHE.get_or_init(|| std::sync::Mutex::new(HashMap::new()))
}

/// Compila ou recupera um regex do cache.
fn get_or_compile_regex(pattern: &str) -> Result<Regex, String> {
    let cache = get_regex_cache();
    let mut map = match cache.lock() {
        Ok(m) => m,
        Err(_) => return Err("regex cache lock poisoned".to_string()),
    };

    if let Some(cached) = map.get(pattern) {
        return cached.clone();
    }

    let result = Regex::new(pattern).map_err(|e| format!("regex inválido: {}", e));
    map.insert(pattern.to_string(), result.clone());
    result
}

/// Regra de validação como definida no schema metadata (Req-2.19.4).
/// Formato flat consumido pelo Pingora a partir do cache DragonflyDB.
#[derive(Debug, Clone, Deserialize)]
pub struct ValidationRule {
    pub field: String,
    /// Tipo: "required", "type", "regex", "range", "length", "enum", "cross_field", "jwt_context", "unique_soft"
    pub rule_type: String,
    pub params: serde_json::Value,
    /// Mensagem legível configurada pelo tenant
    pub message: String,
    /// "error" (bloqueia escrita) ou "warning" (escreve + header X-Cascata-Warnings)
    #[serde(default = "default_severity")]
    pub severity: String,
}

fn default_severity() -> String {
    "error".to_string()
}

/// Resultado de uma validação.
#[derive(Debug)]
pub struct ValidationResult {
    pub valid: bool,
    pub violations: Vec<FieldViolation>,
    pub warnings: Vec<FieldViolation>,
    /// Campos marcados como unique_soft que precisam de query ao banco.
    /// Preenchidos apenas quando etapas 1-5 passaram.
    pub unique_checks_pending: Vec<UniqueCheck>,
}

/// Check de unicidade pendente — será executado via query ao YugabyteDB.
#[derive(Debug, Clone)]
pub struct UniqueCheck {
    pub field: String,
    pub value: serde_json::Value,
    pub scope: Option<String>,
    pub message: String,
}

impl ValidationResult {
    /// Constrói a resposta HTTP 422 a partir das violações.
    pub fn to_error_response(&self) -> CascataValidationError {
        CascataValidationError::new(self.violations.clone())
    }
}

/// Executa validações sobre um payload JSON em 6 etapas sequenciais.
///
/// Etapa 1 falha → retorno imediato sem executar etapas 2-6.
/// Etapas 2-5 coletam todas as violações antes de retornar.
/// Etapa 6 executa apenas se 1-5 passaram.
///
/// `jwt_claims` necessário para validações jwt_context (Req-2.19.3).
pub fn validate(
    rules: &[ValidationRule],
    payload: &serde_json::Value,
    jwt_claims: &serde_json::Value,
) -> ValidationResult {
    let mut violations = Vec::new();
    let mut warnings = Vec::new();
    let mut unique_checks = Vec::new();

    // Classificar regras por etapa
    let mut stage1_required = Vec::new();
    let mut stage2_type = Vec::new();
    let mut stage3_field = Vec::new();
    let mut stage4_cross = Vec::new();
    let mut stage5_jwt = Vec::new();
    let mut stage6_unique = Vec::new();

    for rule in rules {
        match rule.rule_type.as_str() {
            "required" => stage1_required.push(rule),
            "type" => stage2_type.push(rule),
            "regex" | "range" | "length" | "enum" => stage3_field.push(rule),
            "cross_field" => stage4_cross.push(rule),
            "jwt_context" => stage5_jwt.push(rule),
            "unique_soft" => stage6_unique.push(rule),
            _ => {} // Tipos desconhecidos ignorados silenciosamente
        }
    }

    // --- ETAPA 1: required ---
    // Short-circuit: campo obrigatório ausente invalida todo o resto.
    for rule in &stage1_required {
        let value = payload.get(&rule.field);
        if let Some(v) = validate_required(value, &rule.field, &rule.message) {
            collect_violation(&mut violations, &mut warnings, v, &rule.severity);
        }
    }
    if !violations.is_empty() {
        return ValidationResult {
            valid: false,
            violations,
            warnings,
            unique_checks_pending: vec![],
        };
    }

    // --- ETAPA 2: type ---
    for rule in &stage2_type {
        let value = payload.get(&rule.field);
        if let Some(v) = validate_type(value, &rule.params, &rule.field, &rule.message) {
            collect_violation(&mut violations, &mut warnings, v, &rule.severity);
        }
    }

    // --- ETAPA 3: regex, range, length, enum ---
    for rule in &stage3_field {
        let value = payload.get(&rule.field);
        let violation = match rule.rule_type.as_str() {
            "regex" => validate_regex(value, &rule.params, &rule.field, &rule.message),
            "range" => validate_range(value, &rule.params, &rule.field, &rule.message),
            "length" => validate_length(value, &rule.params, &rule.field, &rule.message),
            "enum" => validate_enum(value, &rule.params, &rule.field, &rule.message),
            _ => None,
        };
        if let Some(v) = violation {
            collect_violation(&mut violations, &mut warnings, v, &rule.severity);
        }
    }

    // --- ETAPA 4: cross_field ---
    for rule in &stage4_cross {
        if let Some(v) = validate_cross_field(payload, &rule.params, &rule.field, &rule.message) {
            collect_violation(&mut violations, &mut warnings, v, &rule.severity);
        }
    }

    // --- ETAPA 5: jwt_context ---
    for rule in &stage5_jwt {
        if let Some(v) =
            validate_jwt_context(payload, jwt_claims, &rule.params, &rule.field, &rule.message)
        {
            collect_violation(&mut violations, &mut warnings, v, &rule.severity);
        }
    }

    // Se etapas 2-5 tiveram violações, retornar sem executar etapa 6
    if !violations.is_empty() {
        return ValidationResult {
            valid: false,
            violations,
            warnings,
            unique_checks_pending: vec![],
        };
    }

    // --- ETAPA 6: unique_soft ---
    // Apenas registra quais checks precisam ser feitos — a query real
    // será executada pelo caller que tem conexão ao banco.
    for rule in &stage6_unique {
        let value = payload.get(&rule.field);
        if let Some(val) = value {
            if !val.is_null() {
                unique_checks.push(UniqueCheck {
                    field: rule.field.clone(),
                    value: val.clone(),
                    scope: rule
                        .params
                        .get("scope")
                        .and_then(|s| s.as_str())
                        .map(|s| s.to_string()),
                    message: rule.message.clone(),
                });
            }
        }
    }

    ValidationResult {
        valid: violations.is_empty(),
        violations,
        warnings,
        unique_checks_pending: unique_checks,
    }
}

/// Encaminha violação para a lista correta baseado na severidade.
fn collect_violation(
    violations: &mut Vec<FieldViolation>,
    warnings: &mut Vec<FieldViolation>,
    violation: FieldViolation,
    severity: &str,
) {
    if severity == "warning" {
        warnings.push(violation);
    } else {
        violations.push(violation);
    }
}

// ===========================================================================
// VALIDADORES INDIVIDUAIS
// ===========================================================================

/// Etapa 1: Campo obrigatório presente e não-vazio.
fn validate_required(
    value: Option<&serde_json::Value>,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    match value {
        None | Some(serde_json::Value::Null) => Some(FieldViolation {
            field: field.to_string(),
            rule: "required".to_string(),
            message: message.to_string(),
            value_received: None,
            expression: None,
        }),
        Some(serde_json::Value::String(s)) if s.is_empty() => Some(FieldViolation {
            field: field.to_string(),
            rule: "required".to_string(),
            message: message.to_string(),
            value_received: Some(String::new()),
            expression: None,
        }),
        _ => None,
    }
}

/// Etapa 2: Tipo correto do campo.
/// Verifica se o valor JSON corresponde ao tipo esperado da coluna (params.expected_type).
/// Tipos suportados: "text", "integer", "numeric", "boolean", "uuid", "jsonb", "timestamptz", "date"
fn validate_type(
    value: Option<&serde_json::Value>,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let val = match value {
        // Campo ausente ou null é válido para type check (required cuida disso)
        None | Some(serde_json::Value::Null) => return None,
        Some(v) => v,
    };

    let expected = match params.get("expected_type").and_then(|t| t.as_str()) {
        Some(t) => t,
        None => return None, // Sem tipo esperado configurado — skip
    };

    let type_ok = match expected {
        "text" | "varchar" | "char" => val.is_string(),
        "integer" | "int4" | "int8" | "bigint" | "smallint" => {
            val.is_i64() || val.is_u64() || (val.is_f64() && is_integer_f64(val))
        }
        "numeric" | "decimal" | "float4" | "float8" | "double precision" => val.is_number(),
        "boolean" | "bool" => val.is_boolean(),
        "uuid" => match val.as_str() {
            Some(s) => is_valid_uuid(s),
            None => false,
        },
        "jsonb" | "json" => val.is_object() || val.is_array(),
        "timestamptz" | "timestamp" | "date" | "time" => {
            // Aceita string em formato ISO ou numérico (epoch)
            val.is_string() || val.is_number()
        }
        _ => true, // Tipos desconhecidos passam validação de tipo
    };

    if !type_ok {
        let received = format_value_received(val);
        return Some(FieldViolation {
            field: field.to_string(),
            rule: "type".to_string(),
            message: message.to_string(),
            value_received: Some(received),
            expression: None,
        });
    }

    None
}

/// Verifica se um f64 é efetivamente inteiro (sem parte decimal).
fn is_integer_f64(val: &serde_json::Value) -> bool {
    val.as_f64()
        .map(|f| f == (f as i64) as f64)
        .unwrap_or(false)
}

/// Validação básica de formato UUID (8-4-4-4-12 hex chars).
fn is_valid_uuid(s: &str) -> bool {
    let parts: Vec<&str> = s.split('-').collect();
    if parts.len() != 5 {
        return false;
    }
    let expected_lens = [8, 4, 4, 4, 12];
    parts
        .iter()
        .zip(expected_lens.iter())
        .all(|(part, &len)| part.len() == len && part.chars().all(|c| c.is_ascii_hexdigit()))
}

/// Etapa 3: Regex pattern match.
/// Usa cache estático para evitar recompilação a cada request.
fn validate_regex(
    value: Option<&serde_json::Value>,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let s = value?.as_str()?;
    let pattern = params.get("pattern")?.as_str()?;

    match get_or_compile_regex(pattern) {
        Ok(re) => {
            if !re.is_match(s) {
                return Some(FieldViolation {
                    field: field.to_string(),
                    rule: "regex".to_string(),
                    message: message.to_string(),
                    value_received: Some(s.to_string()),
                    expression: None,
                });
            }
        }
        Err(_) => {
            // Regex inválido configurado pelo tenant — log e skip.
            // Não retorna violação para o cliente por erro de configuração.
            tracing::warn!(
                field = field,
                pattern = pattern,
                "regex inválido configurado em validação — regra ignorada"
            );
        }
    }

    None
}

/// Etapa 3: Range numérico (min/max).
fn validate_range(
    value: Option<&serde_json::Value>,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let v = match value?.as_f64() {
        Some(n) => n,
        None => return None,
    };

    if let Some(min) = params.get("min").and_then(|m| m.as_f64()) {
        if v < min {
            return Some(FieldViolation {
                field: field.to_string(),
                rule: "range".to_string(),
                message: message.to_string(),
                value_received: Some(v.to_string()),
                expression: None,
            });
        }
    }

    if let Some(max) = params.get("max").and_then(|m| m.as_f64()) {
        if v > max {
            return Some(FieldViolation {
                field: field.to_string(),
                rule: "range".to_string(),
                message: message.to_string(),
                value_received: Some(v.to_string()),
                expression: None,
            });
        }
    }

    None
}

/// Etapa 3: Tamanho de string (min/max).
/// Usa chars().count() para contagem correta de caracteres Unicode.
fn validate_length(
    value: Option<&serde_json::Value>,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let s = value?.as_str()?;
    // Usar chars().count() para contagem correta de caracteres Unicode
    let len = s.chars().count() as u64;

    if let Some(min) = params.get("min").and_then(|m| m.as_u64()) {
        if len < min {
            return Some(FieldViolation {
                field: field.to_string(),
                rule: "length".to_string(),
                message: message.to_string(),
                value_received: Some(s.to_string()),
                expression: None,
            });
        }
    }

    if let Some(max) = params.get("max").and_then(|m| m.as_u64()) {
        if len > max {
            return Some(FieldViolation {
                field: field.to_string(),
                rule: "length".to_string(),
                message: message.to_string(),
                value_received: Some(s.to_string()),
                expression: None,
            });
        }
    }

    None
}

/// Etapa 3: Valor deve estar na lista permitida (enum).
fn validate_enum(
    value: Option<&serde_json::Value>,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let v = value?.as_str()?;
    let allowed = params.get("values")?.as_array()?;

    let is_valid = allowed.iter().any(|a| a.as_str() == Some(v));
    if !is_valid {
        return Some(FieldViolation {
            field: field.to_string(),
            rule: "enum".to_string(),
            message: message.to_string(),
            value_received: Some(v.to_string()),
            expression: None,
        });
    }

    None
}

/// Etapa 4: Cross-field validation — expressão envolvendo múltiplos campos.
/// Usa expression evaluator (zero eval, zero injeção).
fn validate_cross_field(
    payload: &serde_json::Value,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let expr = params.get("expression")?.as_str()?;
    let result = expression::evaluate(expr, payload);

    if !result.is_truthy() {
        return Some(FieldViolation {
            field: field.to_string(),
            rule: "cross_field".to_string(),
            message: message.to_string(),
            value_received: None,
            expression: Some(expr.to_string()),
        });
    }

    None
}

/// Etapa 5: JWT Context validation — expressão com claims do JWT.
/// Constrói contexto combinado: payload + jwt claims.
fn validate_jwt_context(
    payload: &serde_json::Value,
    jwt_claims: &serde_json::Value,
    params: &serde_json::Value,
    field: &str,
    message: &str,
) -> Option<FieldViolation> {
    let expr = params.get("expression")?.as_str()?;

    // Construir contexto combinado: payload + jwt claims
    let mut context = serde_json::Map::new();
    if let serde_json::Value::Object(p) = payload {
        for (k, v) in p {
            context.insert(k.clone(), v.clone());
        }
    }
    context.insert("jwt".to_string(), jwt_claims.clone());

    let result = expression::evaluate(expr, &serde_json::Value::Object(context));

    if !result.is_truthy() {
        return Some(FieldViolation {
            field: field.to_string(),
            rule: "jwt_context".to_string(),
            message: message.to_string(),
            value_received: None,
            expression: Some(expr.to_string()),
        });
    }

    None
}

/// Formata um valor JSON para exibição na resposta de erro.
/// Nunca expõe objetos/arrays completos — apenas valores primitivos.
fn format_value_received(val: &serde_json::Value) -> String {
    match val {
        serde_json::Value::String(s) => s.clone(),
        serde_json::Value::Number(n) => n.to_string(),
        serde_json::Value::Bool(b) => b.to_string(),
        serde_json::Value::Null => "null".to_string(),
        serde_json::Value::Array(_) => "[array]".to_string(),
        serde_json::Value::Object(_) => "[object]".to_string(),
    }
}

// ===========================================================================
// TESTES
// ===========================================================================

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn make_rule(field: &str, rule_type: &str, params: serde_json::Value, msg: &str) -> ValidationRule {
        ValidationRule {
            field: field.to_string(),
            rule_type: rule_type.to_string(),
            params,
            message: msg.to_string(),
            severity: "error".to_string(),
        }
    }

    fn make_warning_rule(field: &str, rule_type: &str, params: serde_json::Value, msg: &str) -> ValidationRule {
        ValidationRule {
            field: field.to_string(),
            rule_type: rule_type.to_string(),
            params,
            message: msg.to_string(),
            severity: "warning".to_string(),
        }
    }

    fn empty_jwt() -> serde_json::Value {
        json!({})
    }

    // --- ETAPA 1: required ---

    #[test]
    fn test_required_missing_field() {
        let rules = vec![make_rule("email", "required", json!({}), "Email obrigatório")];
        let payload = json!({});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations.len(), 1);
        assert_eq!(result.violations[0].field, "email");
        assert_eq!(result.violations[0].rule, "required");
    }

    #[test]
    fn test_required_null_value() {
        let rules = vec![make_rule("email", "required", json!({}), "Email obrigatório")];
        let payload = json!({"email": null});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations.len(), 1);
    }

    #[test]
    fn test_required_empty_string() {
        let rules = vec![make_rule("email", "required", json!({}), "Email obrigatório")];
        let payload = json!({"email": ""});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
    }

    #[test]
    fn test_required_present() {
        let rules = vec![make_rule("email", "required", json!({}), "Email obrigatório")];
        let payload = json!({"email": "user@test.com"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
        assert!(result.violations.is_empty());
    }

    #[test]
    fn test_required_short_circuit() {
        // Se required falha, etapa 2+ não deve executar
        let rules = vec![
            make_rule("email", "required", json!({}), "Email obrigatório"),
            make_rule("email", "regex", json!({"pattern": "^.+@.+"}), "Email inválido"),
        ];
        let payload = json!({});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations.len(), 1); // Apenas a violação required
        assert_eq!(result.violations[0].rule, "required");
    }

    // --- ETAPA 2: type ---

    #[test]
    fn test_type_text_valid() {
        let rules = vec![make_rule("name", "type", json!({"expected_type": "text"}), "Deve ser texto")];
        let payload = json!({"name": "João"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_type_text_invalid() {
        let rules = vec![make_rule("name", "type", json!({"expected_type": "text"}), "Deve ser texto")];
        let payload = json!({"name": 123});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "type");
    }

    #[test]
    fn test_type_integer_valid() {
        let rules = vec![make_rule("age", "type", json!({"expected_type": "integer"}), "Deve ser inteiro")];
        let payload = json!({"age": 25});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_type_integer_invalid() {
        let rules = vec![make_rule("age", "type", json!({"expected_type": "integer"}), "Deve ser inteiro")];
        let payload = json!({"age": "vinte e cinco"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
    }

    #[test]
    fn test_type_boolean_valid() {
        let rules = vec![make_rule("active", "type", json!({"expected_type": "boolean"}), "Deve ser boolean")];
        let payload = json!({"active": true});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_type_uuid_valid() {
        let rules = vec![make_rule("id", "type", json!({"expected_type": "uuid"}), "Deve ser UUID")];
        let payload = json!({"id": "550e8400-e29b-41d4-a716-446655440000"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_type_uuid_invalid() {
        let rules = vec![make_rule("id", "type", json!({"expected_type": "uuid"}), "Deve ser UUID")];
        let payload = json!({"id": "not-a-uuid"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
    }

    #[test]
    fn test_type_null_passes() {
        // Null é válido para type check — required cuida da obrigatoriedade
        let rules = vec![make_rule("name", "type", json!({"expected_type": "text"}), "Deve ser texto")];
        let payload = json!({"name": null});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    // --- ETAPA 3: regex ---

    #[test]
    fn test_regex_email_valid() {
        let rules = vec![make_rule(
            "email",
            "regex",
            json!({"pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"}),
            "Email inválido",
        )];
        let payload = json!({"email": "user@example.com"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_regex_email_invalid() {
        let rules = vec![make_rule(
            "email",
            "regex",
            json!({"pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"}),
            "Email inválido",
        )];
        let payload = json!({"email": "nao-e-um-email"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "regex");
        assert_eq!(result.violations[0].value_received.as_deref(), Some("nao-e-um-email"));
    }

    #[test]
    fn test_regex_absent_field() {
        // Campo ausente não é erro de regex — é erro de required
        let rules = vec![make_rule("email", "regex", json!({"pattern": "^.+@.+$"}), "Email inválido")];
        let payload = json!({});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid); // Ausência não viola regex
    }

    // --- ETAPA 3: range ---

    #[test]
    fn test_range_within() {
        let rules = vec![make_rule("qty", "range", json!({"min": 1, "max": 100}), "Fora do range")];
        let payload = json!({"qty": 50});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_range_below_min() {
        let rules = vec![make_rule("qty", "range", json!({"min": 1, "max": 100}), "Fora do range")];
        let payload = json!({"qty": 0});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "range");
    }

    #[test]
    fn test_range_above_max() {
        let rules = vec![make_rule("qty", "range", json!({"min": 1, "max": 100}), "Fora do range")];
        let payload = json!({"qty": 101});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
    }

    // --- ETAPA 3: length ---

    #[test]
    fn test_length_valid() {
        let rules = vec![make_rule("name", "length", json!({"min": 2, "max": 50}), "Tamanho inválido")];
        let payload = json!({"name": "João"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_length_too_short() {
        let rules = vec![make_rule("name", "length", json!({"min": 2, "max": 50}), "Tamanho inválido")];
        let payload = json!({"name": "A"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "length");
    }

    #[test]
    fn test_length_unicode() {
        // "José" = 4 chars, "J\u{00f3}se" = 4 chars. len() em bytes daria 5.
        let rules = vec![make_rule("name", "length", json!({"min": 1, "max": 4}), "Tamanho inválido")];
        let payload = json!({"name": "José"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid); // 4 chars correto via chars().count()
    }

    // --- ETAPA 3: enum ---

    #[test]
    fn test_enum_valid() {
        let rules = vec![make_rule(
            "status",
            "enum",
            json!({"values": ["pendente", "aprovado", "cancelado"]}),
            "Status inválido",
        )];
        let payload = json!({"status": "aprovado"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_enum_invalid() {
        let rules = vec![make_rule(
            "status",
            "enum",
            json!({"values": ["pendente", "aprovado", "cancelado"]}),
            "Status inválido",
        )];
        let payload = json!({"status": "expirado"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "enum");
        assert_eq!(result.violations[0].value_received.as_deref(), Some("expirado"));
    }

    // --- ETAPA 4: cross_field ---

    #[test]
    fn test_cross_field_end_after_start() {
        let rules = vec![make_rule(
            "end_date",
            "cross_field",
            json!({"expression": "end_date > start_date"}),
            "Data de término deve ser posterior à data de início",
        )];
        let payload = json!({"start_date": 10, "end_date": 20});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    #[test]
    fn test_cross_field_end_before_start() {
        let rules = vec![make_rule(
            "end_date",
            "cross_field",
            json!({"expression": "end_date > start_date"}),
            "Data de término deve ser posterior",
        )];
        let payload = json!({"start_date": 20, "end_date": 10});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "cross_field");
        assert!(result.violations[0].expression.is_some());
    }

    #[test]
    fn test_cross_field_total_calculation() {
        let rules = vec![make_rule(
            "total",
            "cross_field",
            json!({"expression": "total == quantity * unit_price"}),
            "Total deve ser igual a qty × preço",
        )];
        let payload = json!({"quantity": 5, "unit_price": 10, "total": 50});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid);
    }

    // --- ETAPA 5: jwt_context ---

    #[test]
    fn test_jwt_context_role_check() {
        let rules = vec![make_rule(
            "_table",
            "jwt_context",
            json!({"expression": "jwt.role IN [\"admin\", \"manager\"]"}),
            "Apenas admins e managers podem escrever",
        )];
        let payload = json!({});
        let jwt = json!({"role": "admin", "sub": "user-123"});
        let result = validate(&rules, &payload, &jwt);
        assert!(result.valid);
    }

    #[test]
    fn test_jwt_context_role_denied() {
        let rules = vec![make_rule(
            "_table",
            "jwt_context",
            json!({"expression": "jwt.role IN [\"admin\", \"manager\"]"}),
            "Apenas admins e managers podem escrever",
        )];
        let payload = json!({});
        let jwt = json!({"role": "viewer", "sub": "user-456"});
        let result = validate(&rules, &payload, &jwt);
        assert!(!result.valid);
        assert_eq!(result.violations[0].rule, "jwt_context");
    }

    #[test]
    fn test_jwt_context_owner_check() {
        let rules = vec![make_rule(
            "owner_id",
            "jwt_context",
            json!({"expression": "owner_id == jwt.sub"}),
            "Apenas o próprio usuário pode criar com seu ID",
        )];
        let payload = json!({"owner_id": "user-123"});
        let jwt = json!({"sub": "user-123"});
        let result = validate(&rules, &payload, &jwt);
        assert!(result.valid);
    }

    #[test]
    fn test_jwt_context_owner_denied() {
        let rules = vec![make_rule(
            "owner_id",
            "jwt_context",
            json!({"expression": "owner_id == jwt.sub"}),
            "Apenas o próprio usuário pode criar com seu ID",
        )];
        let payload = json!({"owner_id": "user-456"});
        let jwt = json!({"sub": "user-123"});
        let result = validate(&rules, &payload, &jwt);
        assert!(!result.valid);
    }

    // --- Severidade: warning vs error ---

    #[test]
    fn test_warning_does_not_block() {
        let rules = vec![make_warning_rule(
            "notes",
            "length",
            json!({"max": 10}),
            "Notas muito longas",
        )];
        let payload = json!({"notes": "Este texto é bem longo e ultrapassará o limite"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid); // Warning não bloqueia
        assert_eq!(result.warnings.len(), 1);
        assert_eq!(result.violations.len(), 0);
    }

    // --- Etapa 6: unique_soft ---

    #[test]
    fn test_unique_soft_registers_check() {
        let rules = vec![make_rule(
            "email",
            "unique_soft",
            json!({}),
            "Email já existe",
        )];
        let payload = json!({"email": "user@test.com"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(result.valid); // Engine não faz a query — apenas registra
        assert_eq!(result.unique_checks_pending.len(), 1);
        assert_eq!(result.unique_checks_pending[0].field, "email");
    }

    #[test]
    fn test_unique_soft_skipped_if_other_violations() {
        let rules = vec![
            make_rule("email", "required", json!({}), "Email obrigatório"),
            make_rule("email", "unique_soft", json!({}), "Email já existe"),
        ];
        let payload = json!({});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations.len(), 1);
        assert_eq!(result.violations[0].rule, "required");
        assert_eq!(result.unique_checks_pending.len(), 0); // Não chegou na etapa 6
    }

    // --- Múltiplas violações ---

    #[test]
    fn test_multiple_violations_same_response() {
        let rules = vec![
            make_rule("email", "regex", json!({"pattern": "^.+@.+$"}), "Email inválido"),
            make_rule("qty", "range", json!({"min": 1}), "Quantidade mínima 1"),
            make_rule("status", "enum", json!({"values": ["ativo", "inativo"]}), "Status inválido"),
        ];
        let payload = json!({"email": "invalido", "qty": 0, "status": "deletado"});
        let result = validate(&rules, &payload, &empty_jwt());
        assert!(!result.valid);
        assert_eq!(result.violations.len(), 3); // Todas coletadas
    }

    // --- Resposta de erro ---

    #[test]
    fn test_error_response_format() {
        let rules = vec![make_rule("email", "required", json!({}), "Email é obrigatório")];
        let payload = json!({});
        let result = validate(&rules, &payload, &empty_jwt());
        let error = result.to_error_response();
        assert_eq!(error.error, "validation_failed");
        assert_eq!(error.violations.len(), 1);

        // Serializa para verificar formato JSON
        let json_str = serde_json::to_string(&error).expect("serialize");
        assert!(json_str.contains("\"validation_failed\""));
        assert!(json_str.contains("\"email\""));
    }
}
