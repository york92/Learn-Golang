package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBPath     string
	ServerPort string
	LogLevel   string
}

func Load() *Config {
	return &Config{
		DBPath:     getEnv("DB_PATH", "./inventory.db"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
