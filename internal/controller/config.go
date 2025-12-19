package controller

import (
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for the UpdateController
type Config struct {
	CheckInterval time.Duration
	SteamCMDPath  string
	SteamApp      string
	SteamAppID    string
	GameMountPath string
	UpdateScript  string
	PodSelector   string
	MaxRetries    int
	RetryDelay    time.Duration
	Namespace     string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		CheckInterval: getEnvDuration("CHECK_INTERVAL", 30*time.Minute),
		SteamCMDPath:  getEnv("STEAMCMD_PATH", "/home/steam/steamcmd"),
		SteamApp:      getEnv("STEAMAPP", "tf"),
		SteamAppID:    getEnv("STEAMAPPID", "232250"),
		GameMountPath: getEnv("GAME_MOUNT_PATH", "/tf"),
		UpdateScript:  getEnv("UPDATE_SCRIPT", "tf_update.txt"),
		PodSelector:   getEnv("POD_SELECTOR", "app=tf2-server"),
		MaxRetries:    getEnvInt("MAX_RETRIES", 3),
		RetryDelay:    getEnvDuration("RETRY_DELAY", 5*time.Minute),
		Namespace:     getEnv("NAMESPACE", "default"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
