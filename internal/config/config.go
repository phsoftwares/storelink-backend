package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL, JWTSecret, Port, AllowedOrigin string
	AdminEmail, AdminPassword, AdminName        string
	Lease, OfflineAfter, RequestTimeout         time.Duration
	MaxAttempts, DefaultBatchSize               int
	RetryDelays                                 []time.Duration
	Swagger                                     bool
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: os.Getenv("JWT_SECRET"), AdminEmail: strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))), AdminPassword: os.Getenv("ADMIN_PASSWORD"), AdminName: strings.TrimSpace(os.Getenv("ADMIN_NAME")), Port: env("PORT", "8080"), AllowedOrigin: env("ALLOWED_ORIGIN", "http://127.0.0.1:5173"), Swagger: env("SWAGGER_ENABLED", "true") == "true"}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL obrigatório")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET deve ter pelo menos 32 caracteres")
	}
	adminConfigured := c.AdminEmail != "" || c.AdminPassword != "" || c.AdminName != ""
	if adminConfigured && (c.AdminEmail == "" || c.AdminPassword == "" || c.AdminName == "" || len(c.AdminPassword) < 12 || len(c.AdminPassword) > 72) {
		return c, fmt.Errorf("ADMIN_EMAIL, ADMIN_NAME e ADMIN_PASSWORD (12 a 72 caracteres) devem ser informados juntos")
	}
	var err error
	// A local FNTS_Server may need several minutes to apply one 500-item
	// page (Firebird triggers and reference remapping are part of the same
	// commit). The lease is only a recovery guard; the agent must still
	// acknowledge after the local commit. Keep the default above the normal
	// page duration so a slow but healthy receiver is not reclaimed halfway
	// through its work. Deployments can still override MESSAGE_LEASE.
	if c.Lease, err = duration("MESSAGE_LEASE", "15m"); err != nil {
		return c, err
	}
	if c.OfflineAfter, err = duration("AGENT_OFFLINE_AFTER", "90s"); err != nil {
		return c, err
	}
	if c.RequestTimeout, err = duration("REQUEST_TIMEOUT", "30s"); err != nil {
		return c, err
	}
	if c.MaxAttempts, err = number("MAX_ATTEMPTS", "5", 1, 100); err != nil {
		return c, err
	}
	if c.DefaultBatchSize, err = number("SYNC_BATCH_SIZE", "500", 1, 1000); err != nil {
		return c, err
	}
	for _, s := range strings.Split(env("RETRY_DELAYS", "5s,30s,2m,10m"), ",") {
		d, e := time.ParseDuration(strings.TrimSpace(s))
		if e != nil || d < time.Second {
			return c, fmt.Errorf("RETRY_DELAYS inválido")
		}
		c.RetryDelays = append(c.RetryDelays, d)
	}
	return c, nil
}
func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func duration(k, d string) (time.Duration, error) {
	v, e := time.ParseDuration(env(k, d))
	if e != nil || v < time.Second {
		return 0, fmt.Errorf("%s inválido", k)
	}
	return v, nil
}
func number(k, d string, min, max int) (int, error) {
	v, e := strconv.Atoi(env(k, d))
	if e != nil || v < min || v > max {
		return 0, fmt.Errorf("%s inválido", k)
	}
	return v, nil
}
