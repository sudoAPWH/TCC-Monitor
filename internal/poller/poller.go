package poller

import (
	"context"
	"log"
	"time"

	"github.com/go-home-iot/honeywell"
	"tcc-monitor/internal/db"
)

type Poller struct {
	thermostat honeywell.Thermostat
	db         *db.DB
	username   string
	password   string
	interval   time.Duration
}

func New(deviceID int, username, password string, interval time.Duration, database *db.DB) *Poller {
	return &Poller{
		thermostat: honeywell.NewThermostat(deviceID),
		db:         database,
		username:   username,
		password:   password,
		interval:   interval,
	}
}

func (p *Poller) Run(ctx context.Context) {
	// Poll once immediately on startup.
	p.poll(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("poller: shutting down")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	if err := p.thermostat.Connect(ctx, p.username, p.password); err != nil {
		log.Printf("poller: connect error: %v", err)
		return
	}

	status, err := p.thermostat.FetchStatus(ctx)
	if err != nil {
		log.Printf("poller: fetch status error: %v", err)
		return
	}

	ui := status.LatestData.UIData
	reading := db.Reading{
		Timestamp:   time.Now().UTC(),
		Temperature: float64(ui.DispTemperature),
		Setpoint:    float64(ui.HeatSetpoint),
	}

	if err := p.db.InsertReading(reading); err != nil {
		log.Printf("poller: insert error: %v", err)
		return
	}

	log.Printf("poller: recorded temp=%.1f setpoint=%.1f",
		reading.Temperature, reading.Setpoint)
}
