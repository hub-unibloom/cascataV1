// Computed Columns — colunas calculadas na API antes da resposta ao cliente.
// Avaliador de expressões Rust com acesso a claims JWT e dados da row.
// Sem eval — avaliador próprio e seguro (expression.rs).
// Ref: SRS Req-2.18.2, Req-2.18.3, SAD §Computed Columns, TASK 0.5.3.

use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::validation::expression;

/// Definição de uma computed column conforme schema metadata (Req-2.18.3).
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ComputedColumnDef {
    /// Nome do campo computado na resposta
    pub name: String,

    /// Tipo de dado retornado: "string", "integer", "numeric", "boolean", "timestamp"
    pub return_type: String,

    /// Tipo de computed column (Req-2.18.3)
    pub kind: ComputedKind,

    /// Expressão a avaliar (ex: "quantity * unit_price", "owner_id == jwt.sub")
    pub expression: String,

    /// Camada onde é executada
    pub layer: ComputedLayer,

    /// Claims JWT necessários para avaliação (apenas api_computed)
    #[serde(default)]
    pub jwt_claims_required: Vec<String>,

    /// Se pode ser usado em WHERE (stored_generated=true, api_computed=false)
    #[serde(default)]
    pub filterable: bool,

    /// Se pode ser usado em ORDER BY (stored_generated=true, api_computed=false)
    #[serde(default)]
    pub sortable: bool,
}

/// Tipo de computed column (Req-2.18.3).
#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum ComputedKind {
    /// Calculada e armazenada no banco (PostgreSQL GENERATED ALWAYS AS ... STORED)
    StoredGenerated,
    /// Calculada na API (Pingora) no momento da resposta
    ApiComputed,
}

/// Camada de execução.
#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum ComputedLayer {
    Database,
    Api,
}

/// Resultado da avaliação de um computed column.
#[derive(Debug)]
pub struct ComputedResult {
    pub name: String,
    pub value: Value,
}

/// Avalia um api_computed column dado o contexto da request.
/// Apenas colunas com kind=ApiComputed são avaliadas aqui.
/// StoredGenerated são calculadas pelo banco — ignoradas.
///
/// # Argumentos
/// - `column`: definição do computed column
/// - `jwt_claims`: claims do JWT do usuário (ex: {"sub": "user123", "role": "admin"})
/// - `row_data`: dados da linha do banco retornada pelo YugabyteDB
///
/// # Retorna
/// Some(ComputedResult) se avaliação bem-sucedida, None se stored_generated ou erro.
pub fn evaluate(
    column: &ComputedColumnDef,
    jwt_claims: &Value,
    row_data: &Value,
) -> Option<ComputedResult> {
    // Stored generated columns são calculadas pelo banco — não processamos aqui
    if column.kind != ComputedKind::ApiComputed {
        return None;
    }

    // Montar contexto com jwt + dados da row
    let context = build_evaluation_context(jwt_claims, row_data);

    // Avaliar a expressão com o evaluator seguro
    let result = expression::evaluate(&column.expression, &context);

    Some(ComputedResult {
        name: column.name.clone(),
        value: result.to_json(),
    })
}

/// Injeta computed columns em um array de rows da resposta.
/// Recebe as definições de computed columns e o response payload,
/// e adiciona os campos calculados em cada row.
///
/// # Argumentos
/// - `columns`: computed column definitions para esta tabela
/// - `rows`: array mutável de rows da resposta
/// - `jwt_claims`: claims do JWT para contexto
pub fn inject_computed_columns(
    columns: &[ComputedColumnDef],
    rows: &mut Vec<Value>,
    jwt_claims: &Value,
) {
    // Filtrar apenas api_computed (stored_generated já estão no banco)
    let api_columns: Vec<&ComputedColumnDef> = columns
        .iter()
        .filter(|c| c.kind == ComputedKind::ApiComputed)
        .collect();

    if api_columns.is_empty() {
        return;
    }

    for row in rows.iter_mut() {
        if let Value::Object(ref mut map) = row {
            for col in &api_columns {
                let context = build_evaluation_context(jwt_claims, &Value::Object(map.clone()));
                let result = expression::evaluate(&col.expression, &context);
                map.insert(col.name.clone(), result.to_json());
            }
        }
    }
}

/// Constrói o contexto de avaliação combinando JWT claims e dados da row.
/// Formato: { "jwt": { ... }, "field1": value1, "field2": value2, ... }
fn build_evaluation_context(jwt_claims: &Value, row_data: &Value) -> Value {
    let mut context = serde_json::Map::new();

    // JWT claims acessíveis via jwt.sub, jwt.role, etc.
    context.insert("jwt".to_string(), jwt_claims.clone());

    // Campos da row acessíveis diretamente pelo nome
    if let Value::Object(row_fields) = row_data {
        for (key, value) in row_fields {
            context.insert(key.clone(), value.clone());
        }
    }

    Value::Object(context)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn make_column(name: &str, expr: &str) -> ComputedColumnDef {
        ComputedColumnDef {
            name: name.to_string(),
            return_type: "numeric".to_string(),
            kind: ComputedKind::ApiComputed,
            expression: expr.to_string(),
            layer: ComputedLayer::Api,
            jwt_claims_required: vec![],
            filterable: false,
            sortable: false,
        }
    }

    #[test]
    fn test_evaluate_api_computed() {
        let col = make_column("total_price", "quantity * unit_price");
        let jwt = json!({"sub": "user1"});
        let row = json!({"quantity": 5, "unit_price": 29.90});

        let result = evaluate(&col, &jwt, &row).unwrap();
        assert_eq!(result.name, "total_price");
        if let Value::Number(n) = result.value {
            assert!((n.as_f64().unwrap() - 149.5).abs() < 0.01);
        } else {
            panic!("Expected number");
        }
    }

    #[test]
    fn test_evaluate_is_owner() {
        let col = ComputedColumnDef {
            name: "is_owner".to_string(),
            return_type: "boolean".to_string(),
            kind: ComputedKind::ApiComputed,
            expression: "owner_id == jwt.sub".to_string(),
            layer: ComputedLayer::Api,
            jwt_claims_required: vec!["sub".to_string()],
            filterable: false,
            sortable: false,
        };

        let jwt = json!({"sub": "user123"});
        let row = json!({"owner_id": "user123", "name": "Test"});

        let result = evaluate(&col, &jwt, &row).unwrap();
        assert_eq!(result.value, json!(true));
    }

    #[test]
    fn test_stored_generated_skipped() {
        let col = ComputedColumnDef {
            name: "slug".to_string(),
            return_type: "string".to_string(),
            kind: ComputedKind::StoredGenerated,
            expression: "lower(title)".to_string(),
            layer: ComputedLayer::Database,
            jwt_claims_required: vec![],
            filterable: true,
            sortable: true,
        };

        let result = evaluate(&col, &json!({}), &json!({}));
        assert!(result.is_none()); // Stored generated → None, banco calcula
    }

    #[test]
    fn test_inject_computed_columns() {
        let cols = vec![make_column("total_price", "quantity * unit_price")];
        let jwt = json!({"sub": "user1"});
        let mut rows = vec![
            json!({"quantity": 5, "unit_price": 10.0}),
            json!({"quantity": 2, "unit_price": 25.0}),
        ];

        inject_computed_columns(&cols, &mut rows, &jwt);

        // Verifica que total_price foi injetado
        assert!(rows[0].get("total_price").is_some());
        assert!(rows[1].get("total_price").is_some());

        let tp0 = rows[0]["total_price"].as_f64().unwrap();
        let tp1 = rows[1]["total_price"].as_f64().unwrap();
        assert!((tp0 - 50.0).abs() < 0.01);
        assert!((tp1 - 50.0).abs() < 0.01);
    }
}
