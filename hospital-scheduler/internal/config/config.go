package config

import (
	"os"
	"strconv"
)

// RuleConfig holds tunable rule parameters (can be overridden via env vars)
type RuleConfig struct {
	// Hard rules
	MaxConsecutiveHours    float64 // default 24
	MinRestBetweenShifts   float64 // hours, default 8
	MinStaffPerSlot        int     // default 1

	// Soft rules
	MaxConsecutiveShifts   int     // default 5
	MaxWeeklyHours         float64 // default 48
	MaxNightShiftsPerMonth int     // default 8
	MaxWorkloadDiffPercent float64 // fairness threshold %, default 20
}

type Config struct {
	DBPath string
	Port   string
	Rules  RuleConfig
}

func Load() *Config {
	return &Config{
		DBPath: getEnv("DB_PATH", "./data/hospital.db"),
		Port:   getEnv("PORT", "8080"),
		Rules: RuleConfig{
			MaxConsecutiveHours:    getFloat("RULE_MAX_CONSECUTIVE_HOURS", 24),
			MinRestBetweenShifts:   getFloat("RULE_MIN_REST_HOURS", 8),
			MinStaffPerSlot:        getInt("RULE_MIN_STAFF_PER_SLOT", 1),
			MaxConsecutiveShifts:   getInt("RULE_MAX_CONSECUTIVE_SHIFTS", 5),
			MaxWeeklyHours:         getFloat("RULE_MAX_WEEKLY_HOURS", 48),
			MaxNightShiftsPerMonth: getInt("RULE_MAX_NIGHT_SHIFTS_MONTH", 8),
			MaxWorkloadDiffPercent: getFloat("RULE_MAX_WORKLOAD_DIFF_PCT", 20),
		},
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
