package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from environment or .env file
type Config struct {
	Port                 string
	TaskTTLDays          int
	CleanupIntervalHours int
}

// Load loads settings from .env file (if present) and environment variables
func Load() *Config {
	loadDotEnv(".env")

	port := getEnv("PORT", "8088")
	ttlDays := getEnvInt("TASK_TTL_DAYS", 30)
	cleanupInterval := getEnvInt("CLEANUP_INTERVAL_HOURS", 6)

	return &Config{
		Port:                 port,
		TaskTTLDays:          ttlDays,
		CleanupIntervalHours: cleanupInterval,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultVal
}

// loadDotEnv parses key=val pairs from a .env file and sets them if not already present
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return // Optional file
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`) // Strip surrounding quotes
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}
