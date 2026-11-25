package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Config struct {
	Address        string
	DatabaseURL    string
	AllowedOrigins []string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{Address: getenv("HTTP_ADDR"), DatabaseURL: getenv("DATABASE_URL")}
	if c.Address == "" {
		c.Address = ":8080"
	}
	_, port, err := net.SplitHostPort(c.Address)
	n, parseErr := strconv.Atoi(port)
	if err != nil || parseErr != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("HTTP_ADDR must contain a valid host:port")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Path == "" || u.Path == "/" {
		return Config{}, errors.New("DATABASE_URL must be a PostgreSQL URL with host and database")
	}
	for _, origin := range strings.Split(getenv("CORS_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return Config{}, errors.New("CORS_ALLOWED_ORIGINS must contain exact HTTP origins without paths")
		}
		c.AllowedOrigins = append(c.AllowedOrigins, origin)
	}
	return c, nil
}
