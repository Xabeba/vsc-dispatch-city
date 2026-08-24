package appenv

import (
	"os"
	"strconv"
	"time"
)

// String reads an environment variable with a fallback.
func String(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Int reads an integer environment variable with a fallback.
func Int(key string, fallback int) int {
	value, err := strconv.Atoi(String(key, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

// DurationMS reads a millisecond duration from an environment variable.
func DurationMS(key string, fallback int) time.Duration {
	return time.Duration(Int(key, fallback)) * time.Millisecond
}
