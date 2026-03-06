// Expression evaluator — avaliador seguro de expressões sem eval().
// Parser e avaliador próprio em Rust para computed columns e validações.
// Zero eval de código arbitrário, zero injeção possível.
// Suporta: aritméticos, comparação, lógicos, acesso a campos, acesso a jwt.
// Ref: SRS Req-2.18.2, Req-2.19.2, SAD §Computed Columns, TASK 0.5.3/0.6.

use serde_json::Value;

/// Resultado de avaliação de uma expressão.
#[derive(Debug, Clone, PartialEq)]
pub enum ExprValue {
    Null,
    Bool(bool),
    Int(i64),
    Float(f64),
    Str(String),
}

impl ExprValue {
    /// Converte ExprValue para serde_json::Value para injeção na resposta.
    pub fn to_json(&self) -> Value {
        match self {
            ExprValue::Null => Value::Null,
            ExprValue::Bool(b) => Value::Bool(*b),
            ExprValue::Int(i) => Value::Number(serde_json::Number::from(*i)),
            ExprValue::Float(f) => {
                serde_json::Number::from_f64(*f)
                    .map(Value::Number)
                    .unwrap_or(Value::Null)
            }
            ExprValue::Str(s) => Value::String(s.clone()),
        }
    }

    /// Tenta converter para f64 para operações aritméticas.
    fn as_f64(&self) -> Option<f64> {
        match self {
            ExprValue::Int(i) => Some(*i as f64),
            ExprValue::Float(f) => Some(*f),
            ExprValue::Str(s) => s.parse::<f64>().ok(),
            _ => None,
        }
    }

    /// Truthiness para operações lógicas.
    pub fn is_truthy(&self) -> bool {
        match self {
            ExprValue::Null => false,
            ExprValue::Bool(b) => *b,
            ExprValue::Int(i) => *i != 0,
            ExprValue::Float(f) => *f != 0.0,
            ExprValue::Str(s) => !s.is_empty(),
        }
    }
}

/// Token do parser de expressões.
#[derive(Debug, Clone, PartialEq)]
enum Token {
    Number(f64),
    Str(String),
    Bool(bool),
    Ident(String),     // campo ou jwt.sub
    Dot,               // .
    Plus,
    Minus,
    Star,
    Slash,
    Eq,                // ==
    Neq,               // !=
    Gt,
    Lt,
    Gte,               // >=
    Lte,               // <=
    And,               // &&
    Or,                // ||
    Not,               // !
    LParen,
    RParen,
    LBracket,          // [
    RBracket,          // ]
    Comma,             // ,
    In,                // IN keyword (Req-2.19.3: jwt.role IN ["admin", "manager"])
    Eof,
}

/// Tokeniza uma expressão em tokens.
fn tokenize(expr: &str) -> Vec<Token> {
    let mut tokens = Vec::new();
    let chars: Vec<char> = expr.chars().collect();
    let len = chars.len();
    let mut i = 0;

    while i < len {
        let c = chars[i];

        // Espaços em branco
        if c.is_whitespace() {
            i += 1;
            continue;
        }

        // Números (inteiros e decimais)
        if c.is_ascii_digit() || (c == '.' && i + 1 < len && chars[i + 1].is_ascii_digit()) {
            let start = i;
            while i < len && (chars[i].is_ascii_digit() || chars[i] == '.') {
                i += 1;
            }
            let num_str: String = chars[start..i].iter().collect();
            if let Ok(n) = num_str.parse::<f64>() {
                tokens.push(Token::Number(n));
            }
            continue;
        }

        // Strings entre aspas
        if c == '"' || c == '\'' {
            let quote = c;
            i += 1;
            let start = i;
            while i < len && chars[i] != quote {
                i += 1;
            }
            let s: String = chars[start..i].iter().collect();
            tokens.push(Token::Str(s));
            if i < len {
                i += 1; // fechar aspas
            }
            continue;
        }

        // Identificadores e keywords
        if c.is_alphabetic() || c == '_' {
            let start = i;
            while i < len && (chars[i].is_alphanumeric() || chars[i] == '_') {
                i += 1;
            }
            let ident: String = chars[start..i].iter().collect();
            match ident.as_str() {
                "true" => tokens.push(Token::Bool(true)),
                "false" => tokens.push(Token::Bool(false)),
                "IN" => tokens.push(Token::In),
                _ => tokens.push(Token::Ident(ident)),
            }
            continue;
        }

        // Operadores de dois caracteres
        if i + 1 < len {
            let two: String = chars[i..=i + 1].iter().collect();
            match two.as_str() {
                "==" => { tokens.push(Token::Eq); i += 2; continue; }
                "!=" => { tokens.push(Token::Neq); i += 2; continue; }
                ">=" => { tokens.push(Token::Gte); i += 2; continue; }
                "<=" => { tokens.push(Token::Lte); i += 2; continue; }
                "&&" => { tokens.push(Token::And); i += 2; continue; }
                "||" => { tokens.push(Token::Or); i += 2; continue; }
                _ => {}
            }
        }

        // Operadores de um caractere
        match c {
            '+' => tokens.push(Token::Plus),
            '-' => tokens.push(Token::Minus),
            '*' => tokens.push(Token::Star),
            '/' => tokens.push(Token::Slash),
            '>' => tokens.push(Token::Gt),
            '<' => tokens.push(Token::Lt),
            '!' => tokens.push(Token::Not),
            '(' => tokens.push(Token::LParen),
            ')' => tokens.push(Token::RParen),
            '[' => tokens.push(Token::LBracket),
            ']' => tokens.push(Token::RBracket),
            ',' => tokens.push(Token::Comma),
            '.' => tokens.push(Token::Dot),
            _ => {} // Caracteres desconhecidos são ignorados
        }
        i += 1;
    }

    tokens.push(Token::Eof);
    tokens
}

/// Parser de expressões recursivo descendente.
struct Parser {
    tokens: Vec<Token>,
    pos: usize,
    context: Value,
}

impl Parser {
    fn new(tokens: Vec<Token>, context: Value) -> Self {
        Parser { tokens, pos: 0, context }
    }

    fn peek(&self) -> &Token {
        self.tokens.get(self.pos).unwrap_or(&Token::Eof)
    }

    fn advance(&mut self) -> Token {
        let tok = self.tokens.get(self.pos).cloned().unwrap_or(Token::Eof);
        self.pos += 1;
        tok
    }

    /// Ponto de entrada: parse da expressão completa.
    fn parse_expr(&mut self) -> ExprValue {
        self.parse_or()
    }

    /// OR: a || b
    fn parse_or(&mut self) -> ExprValue {
        let mut left = self.parse_and();
        while matches!(self.peek(), Token::Or) {
            self.advance();
            let right = self.parse_and();
            left = ExprValue::Bool(left.is_truthy() || right.is_truthy());
        }
        left
    }

    /// AND: a && b
    fn parse_and(&mut self) -> ExprValue {
        let mut left = self.parse_comparison();
        while matches!(self.peek(), Token::And) {
            self.advance();
            let right = self.parse_comparison();
            left = ExprValue::Bool(left.is_truthy() && right.is_truthy());
        }
        left
    }

    /// Comparação: ==, !=, >, <, >=, <=, IN
    fn parse_comparison(&mut self) -> ExprValue {
        let left = self.parse_addition();
        match self.peek().clone() {
            Token::Eq => { self.advance(); let right = self.parse_addition(); self.compare_eq(&left, &right) }
            Token::Neq => { self.advance(); let right = self.parse_addition(); ExprValue::Bool(!self.compare_eq(&left, &right).is_truthy()) }
            Token::Gt => { self.advance(); let right = self.parse_addition(); self.compare_ord(&left, &right, |a, b| a > b) }
            Token::Lt => { self.advance(); let right = self.parse_addition(); self.compare_ord(&left, &right, |a, b| a < b) }
            Token::Gte => { self.advance(); let right = self.parse_addition(); self.compare_ord(&left, &right, |a, b| a >= b) }
            Token::Lte => { self.advance(); let right = self.parse_addition(); self.compare_ord(&left, &right, |a, b| a <= b) }
            Token::In => { self.advance(); self.evaluate_in(&left) }
            _ => left,
        }
    }

    /// Avalia operador IN: left IN [val1, val2, ...]
    /// Suporta: jwt.role IN ["admin", "manager"], status IN ["active", "pending"], x IN [1, 2, 3]
    /// Ref: SRS Req-2.19.3 (jwt.role IN ["admin", "manager"])
    fn evaluate_in(&mut self, left: &ExprValue) -> ExprValue {
        // Espera '[' abrindo a lista
        if !matches!(self.peek(), Token::LBracket) {
            return ExprValue::Bool(false);
        }
        self.advance(); // consume '['

        let mut found = false;
        loop {
            if matches!(self.peek(), Token::RBracket | Token::Eof) {
                break;
            }
            let item = self.parse_primary();
            if self.compare_eq(left, &item).is_truthy() {
                found = true;
            }
            // Consume comma between items
            if matches!(self.peek(), Token::Comma) {
                self.advance();
            }
        }
        // Consume ']'
        if matches!(self.peek(), Token::RBracket) {
            self.advance();
        }

        ExprValue::Bool(found)
    }

    fn compare_eq(&self, left: &ExprValue, right: &ExprValue) -> ExprValue {
        let result = match (left, right) {
            (ExprValue::Null, ExprValue::Null) => true,
            (ExprValue::Bool(a), ExprValue::Bool(b)) => a == b,
            (ExprValue::Str(a), ExprValue::Str(b)) => a == b,
            _ => {
                match (left.as_f64(), right.as_f64()) {
                    (Some(a), Some(b)) => (a - b).abs() < f64::EPSILON,
                    _ => false,
                }
            }
        };
        ExprValue::Bool(result)
    }

    fn compare_ord(&self, left: &ExprValue, right: &ExprValue, op: fn(f64, f64) -> bool) -> ExprValue {
        match (left.as_f64(), right.as_f64()) {
            (Some(a), Some(b)) => ExprValue::Bool(op(a, b)),
            _ => {
                // Comparação de strings
                match (left, right) {
                    (ExprValue::Str(a), ExprValue::Str(b)) => {
                        let cmp = a.cmp(b);
                        let result = match cmp {
                            std::cmp::Ordering::Less => op(-1.0, 0.0),
                            std::cmp::Ordering::Equal => op(0.0, 0.0),
                            std::cmp::Ordering::Greater => op(1.0, 0.0),
                        };
                        ExprValue::Bool(result)
                    }
                    _ => ExprValue::Bool(false),
                }
            }
        }
    }

    /// Adição/subtração: a + b, a - b
    fn parse_addition(&mut self) -> ExprValue {
        let mut left = self.parse_multiplication();
        loop {
            match self.peek() {
                Token::Plus => {
                    self.advance();
                    let right = self.parse_multiplication();
                    // Concatenação de strings
                    if let (ExprValue::Str(a), ExprValue::Str(b)) = (&left, &right) {
                        left = ExprValue::Str(format!("{}{}", a, b));
                    } else {
                        match (left.as_f64(), right.as_f64()) {
                            (Some(a), Some(b)) => left = ExprValue::Float(a + b),
                            _ => left = ExprValue::Null,
                        }
                    }
                }
                Token::Minus => {
                    self.advance();
                    let right = self.parse_multiplication();
                    match (left.as_f64(), right.as_f64()) {
                        (Some(a), Some(b)) => left = ExprValue::Float(a - b),
                        _ => left = ExprValue::Null,
                    }
                }
                _ => break,
            }
        }
        left
    }

    /// Multiplicação/divisão: a * b, a / b
    fn parse_multiplication(&mut self) -> ExprValue {
        let mut left = self.parse_unary();
        loop {
            match self.peek() {
                Token::Star => {
                    self.advance();
                    let right = self.parse_unary();
                    match (left.as_f64(), right.as_f64()) {
                        (Some(a), Some(b)) => left = ExprValue::Float(a * b),
                        _ => left = ExprValue::Null,
                    }
                }
                Token::Slash => {
                    self.advance();
                    let right = self.parse_unary();
                    match (left.as_f64(), right.as_f64()) {
                        (Some(a), Some(b)) if b != 0.0 => left = ExprValue::Float(a / b),
                        _ => left = ExprValue::Null, // Divisão por zero = Null (seguro)
                    }
                }
                _ => break,
            }
        }
        left
    }

    /// Unário: !a, -a
    fn parse_unary(&mut self) -> ExprValue {
        match self.peek() {
            Token::Not => {
                self.advance();
                let val = self.parse_primary();
                ExprValue::Bool(!val.is_truthy())
            }
            Token::Minus => {
                self.advance();
                let val = self.parse_primary();
                match val.as_f64() {
                    Some(n) => ExprValue::Float(-n),
                    None => ExprValue::Null,
                }
            }
            _ => self.parse_primary(),
        }
    }

    /// Primário: literais, identificadores com dot-path, parênteses
    fn parse_primary(&mut self) -> ExprValue {
        let tok = self.advance();
        match tok {
            Token::Number(n) => {
                if n == (n as i64) as f64 && n.abs() < i64::MAX as f64 {
                    ExprValue::Int(n as i64)
                } else {
                    ExprValue::Float(n)
                }
            }
            Token::Str(s) => ExprValue::Str(s),
            Token::Bool(b) => ExprValue::Bool(b),
            Token::Ident(name) => {
                // Resolver dot-path: jwt.sub, jwt.role, row.field, etc.
                let mut path = vec![name];
                while matches!(self.peek(), Token::Dot) {
                    self.advance(); // consume '.'
                    if let Token::Ident(field) = self.advance() {
                        path.push(field);
                    }
                }
                self.resolve_path(&path)
            }
            Token::LParen => {
                let val = self.parse_expr();
                if matches!(self.peek(), Token::RParen) {
                    self.advance();
                }
                val
            }
            _ => ExprValue::Null,
        }
    }

    /// Resolve um caminho de campos no contexto JSON.
    /// Suporta: jwt.sub, jwt.role, owner_id, quantity, etc.
    fn resolve_path(&self, path: &[String]) -> ExprValue {
        let mut current = &self.context;
        for key in path {
            match current.get(key) {
                Some(v) => current = v,
                None => return ExprValue::Null,
            }
        }
        json_to_expr_value(current)
    }
}

/// Converte um serde_json::Value para ExprValue.
fn json_to_expr_value(v: &Value) -> ExprValue {
    match v {
        Value::Null => ExprValue::Null,
        Value::Bool(b) => ExprValue::Bool(*b),
        Value::Number(n) => {
            if let Some(i) = n.as_i64() {
                ExprValue::Int(i)
            } else if let Some(f) = n.as_f64() {
                ExprValue::Float(f)
            } else {
                ExprValue::Null
            }
        }
        Value::String(s) => ExprValue::Str(s.clone()),
        _ => ExprValue::Null, // Arrays/Objects não são resolvidos diretamente
    }
}

/// Avalia uma expressão segura sobre um contexto de dados.
/// Zero eval() — parser e avaliador próprio em Rust.
///
/// O contexto é um JSON com os dados disponíveis:
/// ```json
/// {
///   "jwt": { "sub": "user123", "role": "admin" },
///   "quantity": 5,
///   "unit_price": 29.90,
///   "owner_id": "user123"
/// }
/// ```
///
/// Expressões válidas:
/// - `quantity * unit_price` → 149.5
/// - `owner_id == jwt.sub` → true
/// - `jwt.role == "admin"` → true
/// - `quantity > 0 && unit_price > 0` → true
pub fn evaluate(expr: &str, context: &Value) -> ExprValue {
    let tokens = tokenize(expr);
    let mut parser = Parser::new(tokens, context.clone());
    parser.parse_expr()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_arithmetic() {
        let ctx = json!({"quantity": 5, "unit_price": 29.90});
        let result = evaluate("quantity * unit_price", &ctx);
        if let ExprValue::Float(v) = result {
            assert!((v - 149.5).abs() < 0.01);
        } else {
            panic!("Expected Float, got {:?}", result);
        }
    }

    #[test]
    fn test_comparison_eq() {
        let ctx = json!({"owner_id": "user123", "jwt": {"sub": "user123"}});
        assert_eq!(evaluate("owner_id == jwt.sub", &ctx), ExprValue::Bool(true));
    }

    #[test]
    fn test_comparison_neq() {
        let ctx = json!({"status": "active"});
        assert_eq!(evaluate("status != \"deleted\"", &ctx), ExprValue::Bool(true));
    }

    #[test]
    fn test_comparison_gt() {
        let ctx = json!({"price": 100, "limit": 50});
        assert_eq!(evaluate("price > limit", &ctx), ExprValue::Bool(true));
    }

    #[test]
    fn test_logical_and() {
        let ctx = json!({"a": true, "b": false});
        assert_eq!(evaluate("a && b", &ctx), ExprValue::Bool(false));
    }

    #[test]
    fn test_logical_or() {
        let ctx = json!({"a": true, "b": false});
        assert_eq!(evaluate("a || b", &ctx), ExprValue::Bool(true));
    }

    #[test]
    fn test_string_comparison() {
        let ctx = json!({"jwt": {"role": "admin"}});
        assert_eq!(evaluate("jwt.role == \"admin\"", &ctx), ExprValue::Bool(true));
    }

    #[test]
    fn test_parentheses() {
        let ctx = json!({"a": 2, "b": 3, "c": 4});
        let result = evaluate("(a + b) * c", &ctx);
        if let ExprValue::Float(v) = result {
            assert!((v - 20.0).abs() < 0.01);
        } else {
            panic!("Expected Float, got {:?}", result);
        }
    }

    #[test]
    fn test_division_by_zero() {
        let ctx = json!({"a": 10, "b": 0});
        assert_eq!(evaluate("a / b", &ctx), ExprValue::Null);
    }

    #[test]
    fn test_negation() {
        let ctx = json!({"active": true});
        assert_eq!(evaluate("!active", &ctx), ExprValue::Bool(false));
    }

    #[test]
    fn test_null_field() {
        let ctx = json!({"name": "test"});
        assert_eq!(evaluate("nonexistent_field", &ctx), ExprValue::Null);
    }

    #[test]
    fn test_string_literal() {
        let ctx = json!({});
        assert_eq!(evaluate("\"hello\"", &ctx), ExprValue::Str("hello".into()));
    }

    #[test]
    fn test_complex_expression() {
        // Simula: discounted_price = unit_price * (jwt.role == "vip" ? 0.85 : 1.0)
        // Sem ternário, testamos a comparação parte a parte
        let ctx = json!({"unit_price": 100.0, "jwt": {"role": "vip"}});
        assert_eq!(evaluate("jwt.role == \"vip\"", &ctx), ExprValue::Bool(true));

        let ctx2 = json!({"end_date": 20, "start_date": 10});
        assert_eq!(evaluate("end_date > start_date", &ctx2), ExprValue::Bool(true));
    }

    #[test]
    fn test_concatenation() {
        let ctx = json!({"first_name": "João", "last_name": "Silva"});
        // String concatenation via +
        assert_eq!(
            evaluate("first_name + \" \" + last_name", &ctx),
            ExprValue::Str("João Silva".into())
        );
    }

    // --- Testes do operador IN (Req-2.19.3) ---

    #[test]
    fn test_in_operator_string_match() {
        let ctx = json!({"jwt": {"role": "admin"}});
        assert_eq!(
            evaluate("jwt.role IN [\"admin\", \"manager\"]", &ctx),
            ExprValue::Bool(true)
        );
    }

    #[test]
    fn test_in_operator_string_no_match() {
        let ctx = json!({"jwt": {"role": "viewer"}});
        assert_eq!(
            evaluate("jwt.role IN [\"admin\", \"manager\"]", &ctx),
            ExprValue::Bool(false)
        );
    }

    #[test]
    fn test_in_operator_numeric() {
        let ctx = json!({"status_code": 5});
        assert_eq!(
            evaluate("status_code IN [1, 5, 10]", &ctx),
            ExprValue::Bool(true)
        );
    }

    #[test]
    fn test_in_operator_numeric_no_match() {
        let ctx = json!({"status_code": 7});
        assert_eq!(
            evaluate("status_code IN [1, 5, 10]", &ctx),
            ExprValue::Bool(false)
        );
    }

    #[test]
    fn test_in_operator_empty_list() {
        let ctx = json!({"role": "admin"});
        assert_eq!(
            evaluate("role IN []", &ctx),
            ExprValue::Bool(false)
        );
    }

    #[test]
    fn test_in_operator_with_and() {
        let ctx = json!({"jwt": {"role": "admin"}, "active": true});
        assert_eq!(
            evaluate("jwt.role IN [\"admin\", \"manager\"] && active", &ctx),
            ExprValue::Bool(true)
        );
    }
}
