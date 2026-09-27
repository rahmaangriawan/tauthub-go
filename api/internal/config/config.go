package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config adalah port dari src/lib/config.server.ts (fail-fast di startup).
// Nilai default mengikuti kontrak app lama — lihat REWRITE/01_BACKEND_GO.md §2.
type Config struct {
	AppEnv   string
	AppName  string
	Version  string
	Port     string
	Secret   string

	AccessTokenMinutes  int
	RefreshTokenMinutes int

	DatabaseURL string
	DBHost      string
	DBPort      string
	DBName      string
	DBUser      string
	DBPassword  string
	PoolSize    int32

	AdminEmails []string

	CmsEncryptionSecret     string
	AuthRateLimitPerMinute  int
	WriteRateLimitPerMinute int

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
	PublicAppURL       string

	AIProvider          string
	AIBaseURL           string
	AIAPIKey            string
	AIDefaultModel      string
	AIEconomyModel      string
	AIPremiumModel      string
	AIEmbeddingModel    string
	AIRequestTimeoutSec int
	AIMaxRetries        int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	c := &Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		AppName:     getEnv("APP_NAME", "Tulisio"),
		Version:     getEnv("APP_VERSION", "0.2.0"),
		Port:        getEnv("API_PORT", "8080"),
		Secret:      os.Getenv("APP_SECRET"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		DBHost:      getEnv("DB_HOST", "127.0.0.1"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBName:      os.Getenv("DB_NAME"),
		DBUser:      os.Getenv("DB_USER"),
		DBPassword:  os.Getenv("DB_PASSWORD"),

		CmsEncryptionSecret: os.Getenv("CMS_ENCRYPTION_SECRET"),
	}

	var err error
	if c.AccessTokenMinutes, err = getInt("ACCESS_TOKEN_MINUTES", 30); err != nil {
		return nil, err
	}
	if c.RefreshTokenMinutes, err = getInt("REFRESH_TOKEN_MINUTES", 43200); err != nil {
		return nil, err
	}
	poolSize, err := getInt("DB_POOL_SIZE", 10)
	if err != nil {
		return nil, err
	}
	c.PoolSize = int32(poolSize)
	c.AdminEmails = splitCSV(os.Getenv("PLATFORM_ADMIN_EMAILS"))
	if c.CmsEncryptionSecret == "" {
		c.CmsEncryptionSecret = c.Secret // fallback APP_SECRET (parity config.server.ts)
	}
	if c.AuthRateLimitPerMinute, err = getInt("AUTH_RATE_LIMIT_PER_MINUTE", 10); err != nil {
		return nil, err
	}
	if c.WriteRateLimitPerMinute, err = getInt("WRITE_RATE_LIMIT_PER_MINUTE", 120); err != nil {
		return nil, err
	}

	c.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	c.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	c.GoogleRedirectURI = os.Getenv("GOOGLE_REDIRECT_URI")
	c.PublicAppURL = strings.TrimRight(getEnv("NEXT_PUBLIC_APP_URL", "https://app.tauthub.com"), "/")

	c.AIProvider = getEnv("AI_PROVIDER", "sumopod")
	c.AIBaseURL = os.Getenv("AI_BASE_URL")
	c.AIAPIKey = os.Getenv("AI_API_KEY")
	c.AIDefaultModel = os.Getenv("AI_DEFAULT_MODEL")
	c.AIEconomyModel = os.Getenv("AI_ECONOMY_MODEL")
	c.AIPremiumModel = os.Getenv("AI_PREMIUM_MODEL")
	c.AIEmbeddingModel = os.Getenv("AI_EMBEDDING_MODEL")
	if c.AIRequestTimeoutSec, err = getInt("AI_REQUEST_TIMEOUT_SECONDS", 120); err != nil {
		return nil, err
	}
	if c.AIMaxRetries, err = getInt("AI_MAX_RETRIES", 2); err != nil {
		return nil, err
	}

	if len(c.Secret) < 32 {
		return nil, fmt.Errorf("APP_SECRET wajib >= 32 karakter (dapat %d)", len(c.Secret))
	}
	if c.DatabaseURL == "" && c.DBName == "" {
		return nil, fmt.Errorf("DATABASE_URL atau DB_NAME wajib diisi")
	}
	return c, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

// DBDSN: DATABASE_URL menang, sama seperti client.ts lama.
func (c *Config) DBDSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   c.DBHost + ":" + c.DBPort,
		Path:   c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s harus angka (dapat %q)", key, raw)
	}
	return v, nil
}

func splitCSV(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
