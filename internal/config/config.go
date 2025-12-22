package config

import (
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Server    Server    `yaml:"server"`
	Instagram Instagram `yaml:"instagram"`
	Database  Database  `yaml:"database"`
	Scheduler Scheduler `yaml:"scheduler"`
	S3        S3        `yaml:"s3"`
}

// S3 holds S3/MinIO storage configuration
type S3 struct {
	Endpoint        string `yaml:"endpoint" env:"S3_ENDPOINT" env-default:"http://localhost:9000"`
	AccessKeyID     string `yaml:"access_key_id" env:"S3_ACCESS_KEY_ID" env-default:"minioadmin"`
	SecretAccessKey string `yaml:"secret_access_key" env:"S3_SECRET_ACCESS_KEY" env-default:"minioadmin"`
	Bucket          string `yaml:"bucket" env:"S3_BUCKET" env-default:"media"`
	Region          string `yaml:"region" env:"S3_REGION" env-default:"us-east-1"`
	PublicURL       string `yaml:"public_url" env:"S3_PUBLIC_URL" env-default:"http://localhost:9000/media"`
}

// Server holds HTTP server configuration
type Server struct {
	Host         string        `yaml:"host" env:"SERVER_HOST" env-default:"0.0.0.0"`
	Port         string        `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"SERVER_READ_TIMEOUT" env-default:"15s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"SERVER_WRITE_TIMEOUT" env-default:"15s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"SERVER_IDLE_TIMEOUT" env-default:"60s"`
}

// Address returns the full server address
func (s Server) Address() string {
	return s.Host + ":" + s.Port
}

// Instagram holds Instagram API configuration
type Instagram struct {
	BaseURL    string `yaml:"base_url" env:"INSTAGRAM_BASE_URL" env-default:"https://graph.instagram.com"`
	APIVersion string `yaml:"api_version" env:"INSTAGRAM_API_VERSION" env-default:"v21.0"`
}

// Database holds database configuration
type Database struct {
	// PostgreSQL
	PostgresDSN string `yaml:"postgres_dsn" env:"DATABASE_URL"`

	// Connection pool settings
	MaxOpenConns int           `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	MaxIdleConns int           `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" env-default:"5"`
	ConnLifetime time.Duration `yaml:"conn_lifetime" env:"DB_CONN_LIFETIME" env-default:"5m"`
}

// Scheduler holds scheduler configuration
type Scheduler struct {
	Enabled  bool          `yaml:"enabled" env:"SCHEDULER_ENABLED" env-default:"false"`
	Interval time.Duration `yaml:"interval" env:"SCHEDULER_INTERVAL" env-default:"1m"`
}

// MustLoad loads configuration from environment and panics on error
func MustLoad() Config {
	// Load .env file if exists (for development)
	_ = godotenv.Load()

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	return cfg
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
