// Package metadata — Structs que representam o schema metadata do Cascata.
// Alinhadas 1:1 com DDL em control-plane/migrations/001_initial_schema.sql.
// Lidas por: TableCreator, APIDocs, SDK type gen, MCP Server, Protocol Cascata, Pingora.
// Ref: SRS Req-2.17.7, Req-2.18.3, Req-2.19.4, SAD §Schema Metadata, TASK 0.4/0.5/0.6.
package metadata

import (
	"encoding/json"
	"time"
)

// TableMeta representa uma tabela registrada no schema metadata do Control Plane.
// Campos de soft delete preenchidos apenas quando Status = "recycled".
type TableMeta struct {
	ID          string `json:"id" db:"id"`
	TenantID    string `json:"tenant_id" db:"tenant_id"`
	TableName   string `json:"table_name" db:"table_name"`
	TableSchema string `json:"table_schema" db:"table_schema"`
	Status      string `json:"status" db:"status"` // "active" | "recycled"

	// Campos de soft delete (preenchidos quando status = 'recycled')
	OriginalName     *string    `json:"original_name,omitempty" db:"original_name"`
	OriginalSchema   *string    `json:"original_schema,omitempty" db:"original_schema"`
	RecycledName     *string    `json:"recycled_name,omitempty" db:"recycled_name"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	DeletedBy        *string    `json:"deleted_by,omitempty" db:"deleted_by"`
	ScheduledPurgeAt *time.Time `json:"scheduled_purge_at,omitempty" db:"scheduled_purge_at"`
	RowCountAtDel    *int64     `json:"row_count_at_deletion,omitempty" db:"row_count_at_deletion"`

	// FKs suspensas durante reciclagem — reativadas na restauração (Req-2.17.5)
	SuspendedFKeys json.RawMessage `json:"suspended_fkeys,omitempty" db:"suspended_fkeys"`

	// Resultado da análise de impacto pré-delete (Req-2.17.3)
	ImpactAnalysis json.RawMessage `json:"impact_analysis,omitempty" db:"impact_analysis"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ColumnMeta representa uma coluna no schema metadata.
// Campos obrigatórios desde v1: status, computation, validations, pii, is_writable.
type ColumnMeta struct {
	ID          string `json:"id" db:"id"`
	TableMetaID string `json:"table_meta_id" db:"table_meta_id"`
	TenantID    string `json:"tenant_id" db:"tenant_id"`
	ColumnName  string `json:"column_name" db:"column_name"`
	DataType    string `json:"data_type" db:"data_type"`
	IsNullable  bool   `json:"is_nullable" db:"is_nullable"`
	DefaultVal  *string `json:"default_value,omitempty" db:"default_value"`
	OrdinalPos  int    `json:"ordinal_pos" db:"ordinal_pos"`

	// Campo: status (soft delete awareness — Req-2.17.7)
	Status string `json:"status" db:"status"` // "active" | "recycled"

	// Campo: computation (computed/virtual columns — Req-2.18.3)
	// null = coluna regular
	Computation *Computation `json:"computation,omitempty" db:"computation"`

	// Campo: validations (data validation rules — Req-2.19.4)
	Validations []ValidationRule `json:"validations" db:"validations"`

	// Campo: pii (data masking — Regra 5.5)
	PII bool `json:"pii" db:"pii"`

	// Campo: is_writable (false para computed columns)
	IsWritable bool `json:"is_writable" db:"is_writable"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Computation define como uma coluna é calculada.
// Estrutura alinhada com SRS Req-2.18.3.
type Computation struct {
	// Kind: "stored_generated" (camada banco) ou "api_computed" (camada Pingora)
	Kind string `json:"kind"`

	// Expressão: ex. "quantity * unit_price", "owner_id == jwt.sub"
	Expression string `json:"expression"`

	// Layer: "database" (STORED GENERATED) ou "api" (Pingora injection)
	Layer string `json:"layer"`

	// Claims JWT necessários para avaliação (apenas api_computed)
	JWTClaimsRequired []string `json:"jwt_claims_required,omitempty"`

	// Se pode ser usado em WHERE/ORDER BY (stored_generated=true, api_computed=false)
	Filterable bool `json:"filterable"`
	Sortable   bool `json:"sortable"`
}

// ValidationRule definição de regra de validação por coluna (Req-2.19.4).
type ValidationRule struct {
	Type     string          `json:"type"`     // "required"|"regex"|"range"|"length"|"enum"|"cross_field"|"jwt_context"|"unique_soft"
	Message  string          `json:"message"`  // Mensagem legível
	Severity string          `json:"severity"` // "error" | "warning"
	Params   json.RawMessage `json:"params,omitempty"`
}

// FlatValidationRule formato flat consumido pelo Pingora a partir do cache DragonflyDB.
// Cada regra inclui o nome do campo para que o Pingora não precise cruzar com column_metadata.
// Ref: SRS Req-2.19.4, SAD §Validation Engine.
type FlatValidationRule struct {
	Field    string          `json:"field"`
	RuleType string          `json:"rule_type"`
	Params   json.RawMessage `json:"params,omitempty"`
	Message  string          `json:"message"`
	Severity string          `json:"severity"`
}

// ForeignKeyInfo representa uma FK suspensa durante reciclagem.
type ForeignKeyInfo struct {
	ConstraintName    string `json:"constraint_name"`
	ReferencingSchema string `json:"referencing_schema"`
	ReferencingTable  string `json:"referencing_table"`
	ReferencingColumn string `json:"referencing_column"`
	ReferencedColumn  string `json:"referenced_column"`
}

// ImpactAnalysis resultado da análise de impacto pré-delete (Protocol Cascata).
type ImpactAnalysis struct {
	ForeignKeys  []ForeignKeyInfo  `json:"foreign_keys"`
	ComputedDeps []ComputedDepInfo `json:"computed_deps"`
}

// ComputedDepInfo representa uma coluna computed que depende de outra.
type ComputedDepInfo struct {
	TableName  string `json:"table_name"`
	ColumnName string `json:"column_name"`
	Expression string `json:"expression"`
	Kind       string `json:"kind"` // "stored_generated" | "api_computed"
}

// ConflictStrategy para restauração quando existe conflito de nome (Req-2.17.5).
type ConflictStrategy string

const (
	// ConflictRename restaura com nome alternativo: {nome}_restored_{timestamp}
	ConflictRename ConflictStrategy = "rename"
	// ConflictOverwrite sobrescreve tabela existente (requer confirmação extra)
	ConflictOverwrite ConflictStrategy = "overwrite"
)

// PurgeConfirmation informações de confirmação para purge permanente (Req-2.17.6).
type PurgeConfirmation struct {
	ExecutorID         string `json:"executor_id"`
	ExecutorType       string `json:"executor_type"`        // "member" | "agent" | "system_auto"
	ConfirmationMethod string `json:"confirmation_method"`  // "password" | "passkey"
	IPAddress          string `json:"ip_address"`
}

// TierRetention configuração de retenção por tier (Req-2.17.4).
var TierRetention = map[string]TierRetentionConfig{
	"NANO":       {DefaultDays: 7, MaxDays: 7},
	"MICRO":      {DefaultDays: 14, MaxDays: 30},
	"STANDARD":   {DefaultDays: 30, MaxDays: 90},
	"ENTERPRISE": {DefaultDays: 90, MaxDays: 365},
	"SOVEREIGN":  {DefaultDays: 365, MaxDays: 0}, // 0 = ilimitado
}

// TierRetentionConfig valores de retenção para um tier.
type TierRetentionConfig struct {
	DefaultDays int `json:"default_days"`
	MaxDays     int `json:"max_days"` // 0 = ilimitado
}
