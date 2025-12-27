package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Server     ServerConfig
	Proxy      ProxyConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Encryption EncryptionConfig
	AI         AIConfig
	Stripe     StripeConfig
	Coinbase   CoinbaseConfig
	S3         S3Config
	SMTP       SMTPConfig
}

type ServerConfig struct {
	Port int
	Env  string
}

type ProxyConfig struct {
	Port           int
	DefaultTimeout int // seconds
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	Secret             string
	AccessTokenExpiry  int // minutes
	RefreshTokenExpiry int // hours
}

type EncryptionConfig struct {
	Key string
}

type AIConfig struct {
	OpenAIKey    string
	AnthropicKey string
	GoogleAIKey  string
}

type StripeConfig struct {
	SecretKey     string
	WebhookSecret string
}

type CoinbaseConfig struct {
	APIKey string
}

type S3Config struct {
	Bucket          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
}

type SMTPConfig struct {
	Host      string
	Port      int
	User      string
	Password  string
	FromEmail string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnvInt("API_PORT", 8080),
			Env:  getEnv("APP_ENV", "development"),
		},
		Proxy: ProxyConfig{
			Port:           getEnvInt("PROXY_PORT", 8081),
			DefaultTimeout: getEnvInt("PROXY_TIMEOUT", 30),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agentlink?sslmode=disable"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
		},
		JWT: JWTConfig{
			Secret:             getEnv("JWT_SECRET", ""),
			AccessTokenExpiry:  getEnvInt("JWT_ACCESS_EXPIRY", 15),
			RefreshTokenExpiry: getEnvInt("JWT_REFRESH_EXPIRY", 168), // 7 days
		},
		Encryption: EncryptionConfig{
			Key: getEnv("ENCRYPTION_KEY", ""),
		},
		AI: AIConfig{
			OpenAIKey:    getEnv("OPENAI_API_KEY", ""),
			AnthropicKey: getEnv("ANTHROPIC_API_KEY", ""),
			GoogleAIKey:  getEnv("GOOGLE_AI_API_KEY", ""),
		},
		Stripe: StripeConfig{
			SecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
			WebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		},
		Coinbase: CoinbaseConfig{
			APIKey: getEnv("COINBASE_API_KEY", ""),
		},
		S3: S3Config{
			Bucket:          getEnv("S3_BUCKET", "agentlink-files"),
			Endpoint:        getEnv("S3_ENDPOINT", "http://localhost:9000"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		},
		SMTP: SMTPConfig{
			Host:      getEnv("SMTP_HOST", "localhost"),
			Port:      getEnvInt("SMTP_PORT", 1025),
			User:      getEnv("SMTP_USER", ""),
			Password:  getEnv("SMTP_PASSWORD", ""),
			FromEmail: getEnv("FROM_EMAIL", "noreply@agentlink.io"),
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
	if c.Server.Env == "production" {
		if c.JWT.Secret == "" {
			return fmt.Errorf("JWT_SECRET is required in production")
		}
		if c.Encryption.Key == "" {
			return fmt.Errorf("ENCRYPTION_KEY is required in production")
		}
	}
	return nil
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
