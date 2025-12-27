package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server      ServerConfig
	Proxy       ProxyConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	JWT         JWTConfig
	Encryption  EncryptionConfig
	AI          AIConfig
	Stripe      StripeConfig
	Coinbase    CoinbaseConfig
	S3          S3Config
	SMTP        SMTPConfig
	RateLimit   RateLimitConfig
	Quota       QuotaConfig
	Logging     LoggingConfig
	Monitoring  MonitoringConfig
	CORS        CORSConfig
	Features    FeatureFlags
}

type ServerConfig struct {
	Port         int
	Env          string
	Name         string
	URL          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type ProxyConfig struct {
	Port           int
	URL            string
	DefaultTimeout int // seconds
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type RedisConfig struct {
	URL          string
	MaxRetries   int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type JWTConfig struct {
	Secret             string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	Issuer             string
}

type EncryptionConfig struct {
	Key string
}

type AIConfig struct {
	OpenAIKey       string
	AnthropicKey    string
	GoogleAIKey     string
	DefaultProvider string
	DefaultModel    string
}

type StripeConfig struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
}

type CoinbaseConfig struct {
	APIKey        string
	WebhookSecret string
}

type S3Config struct {
	Bucket          string
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

type SMTPConfig struct {
	Host      string
	Port      int
	User      string
	Password  string
	FromEmail string
	FromName  string
}

type RateLimitConfig struct {
	FreeUserLimit   int
	PaidUserLimit   int
	WindowSeconds   int
}

type QuotaConfig struct {
	FreeInitial      int64
	TrialCallsPerAgent int
}

type LoggingConfig struct {
	Level  string
	Format string
}

type MonitoringConfig struct {
	PrometheusEnabled bool
	PrometheusPort    int
}

type CORSConfig struct {
	AllowedOrigins []string
}

type FeatureFlags struct {
	BlockchainEnabled    bool
	RAGEnabled           bool
	CryptoPaymentEnabled bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvInt("API_PORT", 8080),
			Env:          getEnv("APP_ENV", "development"),
			Name:         getEnv("APP_NAME", "AgentLink"),
			URL:          getEnv("APP_URL", "http://localhost:3000"),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Proxy: ProxyConfig{
			Port:           getEnvInt("PROXY_PORT", 8081),
			URL:            getEnv("PROXY_URL", "http://localhost:8081"),
			DefaultTimeout: getEnvInt("PROXY_TIMEOUT", 30),
		},
		Database: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agentlink?sslmode=disable"),
			MaxConns:        getEnvInt("DB_MAX_CONNS", 25),
			MinConns:        getEnvInt("DB_MIN_CONNS", 5),
			MaxConnLifetime: getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime: getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		},
		Redis: RedisConfig{
			URL:          getEnv("REDIS_URL", "redis://localhost:6379"),
			MaxRetries:   getEnvInt("REDIS_MAX_RETRIES", 3),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		JWT: JWTConfig{
			Secret:             getEnv("JWT_SECRET", ""),
			AccessTokenExpiry:  getEnvDuration("JWT_ACCESS_TOKEN_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: getEnvDuration("JWT_REFRESH_TOKEN_EXPIRY", 168*time.Hour), // 7 days
			Issuer:             getEnv("JWT_ISSUER", "agentlink"),
		},
		Encryption: EncryptionConfig{
			Key: getEnv("ENCRYPTION_KEY", ""),
		},
		AI: AIConfig{
			OpenAIKey:       getEnv("OPENAI_API_KEY", ""),
			AnthropicKey:    getEnv("ANTHROPIC_API_KEY", ""),
			GoogleAIKey:     getEnv("GOOGLE_AI_API_KEY", ""),
			DefaultProvider: getEnv("DEFAULT_AI_PROVIDER", "openai"),
			DefaultModel:    getEnv("DEFAULT_AI_MODEL", "gpt-4"),
		},
		Stripe: StripeConfig{
			SecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
			PublishableKey: getEnv("STRIPE_PUBLISHABLE_KEY", ""),
			WebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
		},
		Coinbase: CoinbaseConfig{
			APIKey:        getEnv("COINBASE_API_KEY", ""),
			WebhookSecret: getEnv("COINBASE_WEBHOOK_SECRET", ""),
		},
		S3: S3Config{
			Bucket:          getEnv("S3_BUCKET", "agentlink-files"),
			Endpoint:        getEnv("S3_ENDPOINT", "http://localhost:9000"),
			Region:          getEnv("S3_REGION", "us-east-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			UsePathStyle:    getEnvBool("S3_USE_PATH_STYLE", true),
		},
		SMTP: SMTPConfig{
			Host:      getEnv("SMTP_HOST", "localhost"),
			Port:      getEnvInt("SMTP_PORT", 1025),
			User:      getEnv("SMTP_USER", ""),
			Password:  getEnv("SMTP_PASSWORD", ""),
			FromEmail: getEnv("SMTP_FROM_EMAIL", "noreply@agentlink.io"),
			FromName:  getEnv("SMTP_FROM_NAME", "AgentLink"),
		},
		RateLimit: RateLimitConfig{
			FreeUserLimit: getEnvInt("RATE_LIMIT_FREE_USER", 10),
			PaidUserLimit: getEnvInt("RATE_LIMIT_PAID_USER", 1000),
			WindowSeconds: getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		},
		Quota: QuotaConfig{
			FreeInitial:        getEnvInt64("FREE_QUOTA_INITIAL", 100),
			TrialCallsPerAgent: getEnvInt("TRIAL_CALLS_PER_AGENT", 3),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "debug"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Monitoring: MonitoringConfig{
			PrometheusEnabled: getEnvBool("PROMETHEUS_ENABLED", true),
			PrometheusPort:    getEnvInt("PROMETHEUS_PORT", 9090),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		},
		Features: FeatureFlags{
			BlockchainEnabled:    getEnvBool("FEATURE_BLOCKCHAIN_ENABLED", false),
			RAGEnabled:           getEnvBool("FEATURE_RAG_ENABLED", true),
			CryptoPaymentEnabled: getEnvBool("FEATURE_CRYPTO_PAYMENT_ENABLED", false),
		},
	}

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required configuration is present
func (c *Config) Validate() error {
	var errs []string

	// Production-only validations
	if c.Server.Env == "production" {
		if c.JWT.Secret == "" {
			errs = append(errs, "JWT_SECRET is required in production")
		}
		if len(c.JWT.Secret) < 32 {
			errs = append(errs, "JWT_SECRET must be at least 32 characters in production")
		}
		if c.Encryption.Key == "" {
			errs = append(errs, "ENCRYPTION_KEY is required in production")
		}
		if len(c.Encryption.Key) < 32 {
			errs = append(errs, "ENCRYPTION_KEY must be at least 32 characters (hex) in production")
		}
	}

	// Database URL validation
	if c.Database.URL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	// Redis URL validation
	if c.Redis.URL == "" {
		errs = append(errs, "REDIS_URL is required")
	}

	// Port validations
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, "API_PORT must be between 1 and 65535")
	}
	if c.Proxy.Port < 1 || c.Proxy.Port > 65535 {
		errs = append(errs, "PROXY_PORT must be between 1 and 65535")
	}

	// Rate limit validations
	if c.RateLimit.FreeUserLimit < 1 {
		errs = append(errs, "RATE_LIMIT_FREE_USER must be at least 1")
	}
	if c.RateLimit.PaidUserLimit < c.RateLimit.FreeUserLimit {
		errs = append(errs, "RATE_LIMIT_PAID_USER must be greater than or equal to RATE_LIMIT_FREE_USER")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}
