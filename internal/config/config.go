package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	AppTitle     string

	MatrixHomeserver   string
	MatrixUsername     string
	MatrixPassword     string
	MatrixRoomID       string
	MatrixPickleKey    string
	MatrixCryptoDBPath string
}

func (c *Config) MatrixEnabled() bool {
	return c.MatrixHomeserver != "" && c.MatrixUsername != "" && c.MatrixPassword != ""
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

	appTitle := os.Getenv("APP_TITLE")
	if appTitle == "" {
		appTitle = "TCC Monitor"
	}

	matrixPickleKey := os.Getenv("MATRIX_PICKLE_KEY")
	if matrixPickleKey == "" {
		matrixPickleKey = "tcc-monitor"
	}

	matrixCryptoDBPath := filepath.Join(filepath.Dir(dbPath), "matrix-crypto.db")

	return &Config{
		TCCUsername:         username,
		TCCPassword:         password,
		TCCDeviceID:         deviceID,
		PollInterval:        pollInterval,
		DBPath:              dbPath,
		ListenAddr:          listenAddr,
		AppTitle:            appTitle,
		MatrixHomeserver:    os.Getenv("MATRIX_HOMESERVER"),
		MatrixUsername:      os.Getenv("MATRIX_USERNAME"),
		MatrixPassword:      os.Getenv("MATRIX_PASSWORD"),
		MatrixRoomID:        os.Getenv("MATRIX_ROOM_ID"),
		MatrixPickleKey:     matrixPickleKey,
		MatrixCryptoDBPath:  matrixCryptoDBPath,
	}, nil
}
