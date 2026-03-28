package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

type Reading struct {
	ID          int64
	Timestamp   time.Time
	Temperature float64
	Setpoint    float64
}

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for concurrent read/write performance.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &DB{conn: conn}, nil
}

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS readings (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp  DATETIME NOT NULL,
			temperature REAL NOT NULL,
			setpoint   REAL NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_readings_timestamp ON readings(timestamp);

		CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS notifications (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			message   TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_notifications_timestamp ON notifications(timestamp);
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Convert any old ISO 8601 timestamps (with T and Z) to SQLite-native format.
	conn.Exec("UPDATE readings SET timestamp = replace(replace(timestamp, 'T', ' '), 'Z', '') WHERE timestamp LIKE '%T%'")
	conn.Exec("UPDATE notifications SET timestamp = replace(replace(timestamp, 'T', ' '), 'Z', '') WHERE timestamp LIKE '%T%'")

	return nil
}

func parseTimestamp(ts string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05+00:00",
	} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (d *DB) InsertReading(r Reading) error {
	_, err := d.conn.Exec(
		"INSERT INTO readings (timestamp, temperature, setpoint) VALUES (?, ?, ?)",
		r.Timestamp.UTC().Format("2006-01-02 15:04:05"), r.Temperature, r.Setpoint,
	)
	return err
}

func (d *DB) Latest() (*Reading, error) {
	row := d.conn.QueryRow(
		"SELECT id, timestamp, temperature, setpoint FROM readings ORDER BY timestamp DESC LIMIT 1",
	)
	var r Reading
	var ts string
	if err := row.Scan(&r.ID, &ts, &r.Temperature, &r.Setpoint); err != nil {
		return nil, err
	}
	r.Timestamp = parseTimestamp(ts)
	return &r, nil
}

func (d *DB) ReadingsSince(since time.Time) ([]Reading, error) {
	rows, err := d.conn.Query(
		"SELECT id, timestamp, temperature, setpoint FROM readings WHERE timestamp >= ? ORDER BY timestamp ASC",
		since.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []Reading
	for rows.Next() {
		var r Reading
		var ts string
		if err := rows.Scan(&r.ID, &ts, &r.Temperature, &r.Setpoint); err != nil {
			return nil, err
		}
		r.Timestamp, _ = time.Parse("2006-01-02 15:04:05+00:00", ts)
		if r.Timestamp.IsZero() {
			r.Timestamp, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		}
		readings = append(readings, r)
	}
	return readings, rows.Err()
}

// ReadingsForDay returns all readings for a specific date (YYYY-MM-DD).
func (d *DB) ReadingsForDay(date string) ([]Reading, error) {
	rows, err := d.conn.Query(
		"SELECT id, timestamp, temperature, setpoint FROM readings WHERE date(timestamp) = ? ORDER BY timestamp ASC",
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []Reading
	for rows.Next() {
		var r Reading
		var ts string
		if err := rows.Scan(&r.ID, &ts, &r.Temperature, &r.Setpoint); err != nil {
			return nil, err
		}
		r.Timestamp, _ = time.Parse("2006-01-02 15:04:05+00:00", ts)
		if r.Timestamp.IsZero() {
			r.Timestamp, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		}
		readings = append(readings, r)
	}
	return readings, rows.Err()
}

// DaysWithData returns a list of date strings (YYYY-MM-DD) that have readings in the given month.
func (d *DB) DaysWithData(year int, month int) ([]string, error) {
	rows, err := d.conn.Query(
		"SELECT DISTINCT date(timestamp) FROM readings WHERE strftime('%Y', timestamp) = ? AND strftime('%m', timestamp) = ? ORDER BY date(timestamp)",
		fmt.Sprintf("%04d", year), fmt.Sprintf("%02d", month),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	return value, err
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.conn.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

func (d *DB) GetThresholds() (low, high float64, err error) {
	if v, e := d.GetSetting("threshold_low"); e == nil {
		low, _ = strconv.ParseFloat(v, 64)
	}
	if v, e := d.GetSetting("threshold_high"); e == nil {
		high, _ = strconv.ParseFloat(v, 64)
	}
	return low, high, nil
}

func (d *DB) SetThresholds(low, high float64) error {
	if err := d.SetSetting("threshold_low", strconv.FormatFloat(low, 'f', 1, 64)); err != nil {
		return err
	}
	return d.SetSetting("threshold_high", strconv.FormatFloat(high, 'f', 1, 64))
}

func (d *DB) GetCooldownMinutes() int {
	if v, err := d.GetSetting("cooldown_minutes"); err == nil {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 30
}

func (d *DB) RecordNotification(message string) error {
	_, err := d.conn.Exec(
		"INSERT INTO notifications (timestamp, message) VALUES (?, ?)",
		time.Now().UTC().Format("2006-01-02 15:04:05"), message,
	)
	return err
}

func (d *DB) GetLastNotificationTime() (time.Time, error) {
	var ts string
	err := d.conn.QueryRow("SELECT timestamp FROM notifications ORDER BY timestamp DESC LIMIT 1").Scan(&ts)
	if err != nil {
		return time.Time{}, err
	}
	return parseTimestamp(ts), nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}
