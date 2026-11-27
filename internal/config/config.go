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
	viper.SetConfigFile(".env")
	viper.ReadInConfig()

	port := viper.Get("PORT").(string)
	ttlDays := viper.Get("TaskTTLDays").(int)
	cleanupInterval := viper.Get("CLEANUP_INTERVAL_HOURS").(int)

	return &Config{
		Port:                 port,
		TaskTTLDays:          ttlDays,
		CleanupIntervalHours: cleanupInterval,
	}
}
