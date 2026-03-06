// Package config carrega a configuração do Control Plane.
// Fonte: YAML file + override por env vars. Zero valor hardcoded.
// Ref: SRS Req-2.1.1 (CP governa ambiente), SAD §A, TASK PR-5.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CascataConfig é a configuração raiz do Control Plane.
// Todo campo sensível (passwords, keys) deve referenciar um path do OpenBao,
// nunca valor direto. Em modo dev, valores de dev são aceitos via config.dev.yaml.
type CascataConfig struct {
	// Banco do Control Plane (schema metadata, catálogo extensões, tenants)
	ControlDB PostgresConfig `yaml:"control_db"`

	// DragonflyDB para cache de metadata
	Cache DragonflyConfig `yaml:"cache"`



	// ClickHouse para observabilidade (logs, traces, analytics)
	Analytics ClickHouseConfig `yaml:"analytics"`

	// Redpanda para eventos assíncronos
	EventStream RedpandaConfig `yaml:"event_stream"`

	// OpenBao para gestão de secrets
	KMS OpenBaoConfig `yaml:"kms"`

	// Configurações de tier e limites por tier
	Tiers TierConfig `yaml:"tiers"`

	// HTTP server do Control Plane
	HTTP HTTPConfig `yaml:"http"`
}

// PostgresConfig — conexão ao YugabyteDB do Control Plane.
// NÃO é o banco de tenant. É o banco de gestão do sistema.
type PostgresConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Database        string        `yaml:"database"`
	User            string        `yaml:"user"`
	PasswordBaoPath string        `yaml:"password_bao_path"` // Path no OpenBao, ex: "secret/cascata/cp/db_password"
	SSLMode         string        `yaml:"ssl_mode"`
	MaxConns        int           `yaml:"max_conns"`
	MinConns        int           `yaml:"min_conns"`
	ConnTimeout     time.Duration `yaml:"conn_timeout"`
}

// DragonflyConfig — cache Redis-compatible.
type DragonflyConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	PasswordBaoPath string        `yaml:"password_bao_path"`
	DB              int           `yaml:"db"`
	PoolSize        int           `yaml:"pool_size"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
}



// ClickHouseConfig — analytics e observabilidade OLAP.
type ClickHouseConfig struct {
	Host            string `yaml:"host"`
	HTTPPort        int    `yaml:"http_port"`
	NativePort      int    `yaml:"native_port"`
	Database        string `yaml:"database"`
	User            string `yaml:"user"`
	PasswordBaoPath string `yaml:"password_bao_path"`
}

// RedpandaConfig — streaming de eventos via protocolo Kafka.
type RedpandaConfig struct {
	Brokers        []string `yaml:"brokers"`
	SchemaRegistry string   `yaml:"schema_registry"`
	AdminAPI       string   `yaml:"admin_api"`
}

// OpenBaoConfig — KMS e gestão de secrets.
type OpenBaoConfig struct {
	Address     string `yaml:"address"`
	TokenPath   string `yaml:"token_path"` // Path do service token (file ou env)
	MountPath   string `yaml:"mount_path"` // Mount path no Bao, ex: "secret/"
	TLSInsecure bool   `yaml:"tls_insecure"` // Apenas para dev
}

// DowngradeThresholds contém as regras de queda de um tier específico para o inferior.
type DowngradeThresholds struct {
	ConsecutiveDaysRequired int     `yaml:"consecutive_days_required"`
	MaxP95RequestsPerHour   float64 `yaml:"max_p95_requests_per_hour"`
	MaxActiveUsers          int64   `yaml:"max_active_users"`
	MaxStorageGB            float64 `yaml:"max_storage_gb"`
	MaxConcurrentConns      float64 `yaml:"max_concurrent_conns"`
}

// TierConfig — limites e comportamentos por tier.
type TierConfig struct {
	DefaultTier     string                         `yaml:"default_tier"`      // "NANO"
	SharedCluster   string                         `yaml:"shared_cluster"`    // "shared" ou "full"
	MaxNanoTenants  int                            `yaml:"max_nano_tenants"`  // Limite no cluster compartilhado
	MaxMicroTenants int                            `yaml:"max_micro_tenants"`
	Downgrades      map[string]DowngradeThresholds `yaml:"downgrades"`
}

// HTTPConfig — servidor HTTP do Control Plane.
type HTTPConfig struct {
	Addr            string        `yaml:"addr"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// Load carrega a configuração do YAML e aplica overrides de env vars.
// Prioridade: env var > YAML file > defaults.
// Path do YAML: env CASCATA_CONFIG_PATH, default "/etc/cascata/config.yaml".
func Load() (*CascataConfig, error) {
	cfg := defaults()

	configPath := os.Getenv("CASCATA_CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/cascata/config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Se o arquivo não existe, usar apenas defaults + env
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("config.Load: lendo %s: %w", configPath, err)
		}
		// Arquivo não encontrado — ok, vai com defaults
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config.Load: parsing YAML %s: %w", configPath, err)
		}
	}

	// Override de env vars para campos críticos de infraestrutura
	applyEnvOverrides(cfg)

	return cfg, nil
}

// defaults retorna configuração com valores sensíveis para dev/Shelter.
func defaults() *CascataConfig {
	return &CascataConfig{
		ControlDB: PostgresConfig{
			Host:        "yugabytedb",
			Port:        5433,
			Database:    "cascata_control",
			User:        "yugabyte",
			SSLMode:     "disable",
			MaxConns:    10,
			MinConns:    2,
			ConnTimeout: 10 * time.Second,
		},
		Cache: DragonflyConfig{
			Host:         "dragonflydb",
			Port:         6379,
			DB:           0,
			PoolSize:     10,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Analytics: ClickHouseConfig{
			Host:       "clickhouse",
			HTTPPort:   8123,
			NativePort: 9001,
			Database:   "cascata_logs",
			User:       "cascata",
		},
		EventStream: RedpandaConfig{
			Brokers:        []string{"redpanda:29092"},
			SchemaRegistry: "http://redpanda:8081",
			AdminAPI:       "http://redpanda:9644",
		},
		KMS: OpenBaoConfig{
			Address:     "http://openbao:8200",
			TokenPath:   "/etc/cascata/bao-token",
			MountPath:   "secret/",
			TLSInsecure: true,
		},
		Tiers: TierConfig{
			DefaultTier:     "NANO",
			SharedCluster:   "shared",
			MaxNanoTenants:  200,
			MaxMicroTenants: 50,
			Downgrades: map[string]DowngradeThresholds{
				"MICRO": {
					ConsecutiveDaysRequired: 30,
					MaxP95RequestsPerHour:   200,
					MaxActiveUsers:          50,
					MaxStorageGB:            0.5,
					MaxConcurrentConns:      5,
				},
				"STANDARD": {
					ConsecutiveDaysRequired: 30,
					MaxP95RequestsPerHour:   1000,
					MaxActiveUsers:          500,
					MaxStorageGB:            2.0,
					MaxConcurrentConns:      25,
				},
				"ENTERPRISE": {
					ConsecutiveDaysRequired: 45,
					MaxP95RequestsPerHour:   10000,
					MaxActiveUsers:          5000,
					MaxStorageGB:            50.0,
					MaxConcurrentConns:      250,
				},
			},
		},
		HTTP: HTTPConfig{
			Addr:            ":9090",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
	}
}

// applyEnvOverrides substitui valores da config com env vars quando presentes.
// Convenção: CASCATA_<SECTION>_<FIELD>, ex: CASCATA_CONTROLDB_HOST.
func applyEnvOverrides(cfg *CascataConfig) {
	if v := os.Getenv("CASCATA_HTTP_ADDR"); v != "" {
		cfg.HTTP.Addr = v
	}
	if v := os.Getenv("CASCATA_CONTROLDB_HOST"); v != "" {
		cfg.ControlDB.Host = v
	}
	if v := os.Getenv("CASCATA_CACHE_HOST"); v != "" {
		cfg.Cache.Host = v
	}
	if v := os.Getenv("CASCATA_ANALYTICS_HOST"); v != "" {
		cfg.Analytics.Host = v
	}
	if v := os.Getenv("CASCATA_KMS_ADDRESS"); v != "" {
		cfg.KMS.Address = v
	}
}
