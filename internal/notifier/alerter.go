package notifier

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"tcc-monitor/internal/db"
)

type MatrixConfig struct {
	Homeserver   string
	Username     string
	Password     string
	PickleKey    string
	CryptoDBPath string
}

type Alerter struct {
	mu          sync.Mutex
	notifier    *Notifier
	matrixCfg   MatrixConfig
	db          *db.DB
	envRoomID   string
}

func NewAlerter(notifier *Notifier, matrixCfg MatrixConfig, database *db.DB, envRoomID string) *Alerter {
	return &Alerter{
		notifier:  notifier,
		matrixCfg: matrixCfg,
		db:        database,
		envRoomID: envRoomID,
	}
}

// ensureNotifier returns the active notifier, attempting to connect if not yet ready.
func (a *Alerter) ensureNotifier(ctx context.Context) *Notifier {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.notifier != nil {
		return a.notifier
	}

	n, err := New(ctx, a.matrixCfg.Homeserver, a.matrixCfg.Username, a.matrixCfg.Password, a.matrixCfg.PickleKey, a.matrixCfg.CryptoDBPath, a.db)
	if err != nil {
		log.Printf("matrix: retry connect failed: %v", err)
		return nil
	}
	log.Println("matrix: connected (retry succeeded)")
	a.notifier = n
	return n
}

func (a *Alerter) CheckReading(ctx context.Context, reading db.Reading) {
	n := a.ensureNotifier(ctx)
	if n == nil {
		return
	}

	low, high, err := a.db.GetThresholds()
	if err != nil {
		return
	}

	// If both thresholds are zero/unset, nothing to check.
	if low == 0 && high == 0 {
		return
	}

	var plain, html string
	temp := reading.Temperature

	if high > 0 && temp > high {
		plain = fmt.Sprintf("\u26a0\ufe0f Temperature Alert: %.1f\u00b0C is above the high threshold of %.1f\u00b0C", temp, high)
		html = fmt.Sprintf("\u26a0\ufe0f <b>Temperature Alert</b><br><br>Current: <b>%.1f\u00b0C</b> — above the high threshold of <b>%.1f\u00b0C</b>", temp, high)
	} else if low > 0 && temp < low {
		plain = fmt.Sprintf("\u2744\ufe0f Temperature Alert: %.1f\u00b0C is below the low threshold of %.1f\u00b0C", temp, low)
		html = fmt.Sprintf("\u2744\ufe0f <b>Temperature Alert</b><br><br>Current: <b>%.1f\u00b0C</b> — below the low threshold of <b>%.1f\u00b0C</b>", temp, low)
	}

	if plain == "" {
		return
	}

	// Check cooldown.
	cooldownMinutes := a.db.GetCooldownMinutes()
	lastNotif, err := a.db.GetLastNotificationTime()
	if err == nil && time.Since(lastNotif) < time.Duration(cooldownMinutes)*time.Minute {
		return
	}

	// Determine room ID: DB setting overrides env var.
	roomID := a.envRoomID
	if dbRoom, err := a.db.GetSetting("matrix_room_id"); err == nil && dbRoom != "" {
		roomID = dbRoom
	}
	if roomID == "" {
		log.Println("alerter: threshold breached but no Matrix room ID configured")
		return
	}

	if err := n.SendAlert(ctx, roomID, plain, html); err != nil {
		log.Printf("alerter: failed to send alert: %v", err)
		return
	}

	if err := a.db.RecordNotification(plain); err != nil {
		log.Printf("alerter: failed to record notification: %v", err)
	}

	log.Printf("alerter: sent alert: %s", plain)
}

func (a *Alerter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.notifier != nil {
		a.notifier.Stop()
		a.notifier = nil
	}
}
