package notifier

import (
	"context"
	"fmt"
	"log"
	"time"

	"tcc-monitor/internal/db"
)

type Alerter struct {
	notifier    *Notifier
	db          *db.DB
	envRoomID   string
}

func NewAlerter(notifier *Notifier, database *db.DB, envRoomID string) *Alerter {
	return &Alerter{
		notifier:  notifier,
		db:        database,
		envRoomID: envRoomID,
	}
}

func (a *Alerter) CheckReading(ctx context.Context, reading db.Reading) {
	if a.notifier == nil {
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

	var message string
	temp := reading.Temperature

	if high > 0 && temp > high {
		message = fmt.Sprintf("Temperature alert: %.1f\u00b0C is above the high threshold of %.1f\u00b0C", temp, high)
	} else if low > 0 && temp < low {
		message = fmt.Sprintf("Temperature alert: %.1f\u00b0C is below the low threshold of %.1f\u00b0C", temp, low)
	}

	if message == "" {
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

	if err := a.notifier.SendAlert(ctx, roomID, message); err != nil {
		log.Printf("alerter: failed to send alert: %v", err)
		return
	}

	if err := a.db.RecordNotification(message); err != nil {
		log.Printf("alerter: failed to record notification: %v", err)
	}

	log.Printf("alerter: sent alert: %s", message)
}
