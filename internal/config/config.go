package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	TCCUsername  string
	TCCPassword string
	TCCDeviceID int
	PollInterval time.Duration
	DBPath       string
	ListenAddr   string
}

func Load() (*Config, error) {
	username := os.Getenv("TCC_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("TCC_USERNAME is required")
	}

	password := os.Getenv("TCC_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("TCC_PASSWORD is required")
	}

	deviceIDStr := os.Getenv("TCC_DEVICE_ID")
	if deviceIDStr == "" {
		return nil, fmt.Errorf("TCC_DEVICE_ID is required")
	}
	deviceID, err := strconv.Atoi(deviceIDStr)
	if err != nil {
		return nil, fmt.Errorf("TCC_DEVICE_ID must be a number: %w", err)
	}

	pollInterval := 10 * time.Minute
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("POLL_INTERVAL invalid duration: %w", err)
		}
		pollInterval = d
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "tcc-monitor.db"
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	return &Config{
		TCCUsername:   username,
		TCCPassword:   password,
		TCCDeviceID:   deviceID,
		PollInterval:  pollInterval,
		DBPath:        dbPath,
		ListenAddr:    listenAddr,
	}, nil
}
