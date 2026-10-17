// settings come from env vars, compose fills them from .env
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() (Config, error) {
	// remember all missing names so the error lists them at once
	var missing []string
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	user := required("POSTGRES_USER")
	password := required("POSTGRES_PASSWORD")
	dbName := required("POSTGRES_DB")
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s (copy .env.example to .env)", strings.Join(missing, ", "))
	}

	host := getenv("DB_HOST", "localhost") // "db" in compose (service name)
	port := getenv("DB_PORT", "5432")

	// net/url escapes weird chars in the password, sprintf would not
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + dbName,
		RawQuery: "sslmode=disable", // no tls, traffic stays inside compose network
	}

	return Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: u.String(),
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
