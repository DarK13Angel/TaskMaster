package config

import (
	"github.com/spf13/viper"
)

// Config holds runtime settings loaded from environment or .env file
type Config struct {
	Port                 string
	TaskTTLDays          int
	CleanupIntervalHours int
}

// Load loads settings from .env file (if present) and environment variables
func Load() *Config {
	viper.SetDefault("PORT", "8088")
	viper.SetDefault("TASK_TTL_DAYS", 30)
	viper.SetDefault("CLEANUP_INTERVAL_HOURS", 6)

	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	return &Config{
		Port:                 viper.GetString("PORT"),
		TaskTTLDays:          viper.GetInt("TASK_TTL_DAYS"),
		CleanupIntervalHours: viper.GetInt("CLEANUP_INTERVAL_HOURS"),
	}
}
