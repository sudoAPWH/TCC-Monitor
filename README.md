# TCC Monitor

A lightweight, self-hosted dashboard for monitoring Honeywell Total Connect Comfort (TCC) thermostats. Polls your thermostat on a configurable interval and stores temperature readings in a local SQLite database, then serves a clean web dashboard with live stats and historical charts. Optionally sends Matrix notifications when temperature thresholds are breached.

## Features

- **Live polling** of temperature and heat setpoint from the TCC API
- **Real-time dashboard** with current stats that auto-refresh via HTMX
- **Temperature trend charts** for the last 24 hours or 7 days (Chart.js)
- **Calendar history view** -- click any day to see that day's readings
- **Matrix notifications** -- get alerted when temperature goes above or below configurable thresholds
- **End-to-end encryption** -- Matrix bot supports E2EE with auto-accept emoji verification
- **SQLite storage** with WAL mode for lightweight, zero-config persistence
- **Docker Compose deployment** -- one command to build and run

## Quick Start

1. **Clone the repo:**

   ```sh
   git clone https://github.com/sudoAPWH/TCC-Monitor.git
   cd TCC-Monitor
   ```

2. **Create your `.env` file:**

   ```sh
   cp .env.example .env
   ```

   Edit `.env` and fill in your TCC credentials and device ID. You can find your device ID in the URL when viewing your thermostat on [mytotalconnectcomfort.com](https://mytotalconnectcomfort.com).

3. **Run with Docker Compose:**

   ```sh
   docker compose up -d
   ```

4. **Open the dashboard** at [http://localhost:8080](http://localhost:8080)

## Configuration

All configuration is done through environment variables (set in `.env`):

| Variable | Required | Default | Description |
|---|---|---|---|
| `TCC_USERNAME` | Yes | -- | Your TCC account email |
| `TCC_PASSWORD` | Yes | -- | Your TCC account password |
| `TCC_DEVICE_ID` | Yes | -- | Thermostat device ID |
| `POLL_INTERVAL` | No | `10m` | How often to poll (Go duration, e.g. `5m`, `30s`) |
| `DB_PATH` | No | `tcc-monitor.db` | Path to the SQLite database file |
| `LISTEN_ADDR` | No | `:8080` | Address and port for the web server |
| `APP_TITLE` | No | `TCC Monitor` | Dashboard title shown in the header and browser tab |
| `TZ` | No | `UTC` | Timezone for displayed timestamps (e.g. `America/Edmonton`) |

### Matrix Notifications (optional)

| Variable | Required | Default | Description |
|---|---|---|---|
| `MATRIX_HOMESERVER` | No | -- | Matrix server URL (e.g. `https://matrix.example.com`) |
| `MATRIX_USERNAME` | No | -- | Bot account username |
| `MATRIX_PASSWORD` | No | -- | Bot account password |
| `MATRIX_ROOM_ID` | No | -- | Room ID to send alerts to (e.g. `!abc123:example.com`) |
| `MATRIX_PICKLE_KEY` | No | `tcc-monitor` | Encryption key for the crypto store |

If `MATRIX_HOMESERVER`, `MATRIX_USERNAME`, and `MATRIX_PASSWORD` are all set, the bot will connect to Matrix and send alerts when thresholds are breached. If any are missing, the app runs normally without notifications.

### Alert Thresholds

Thresholds are configured from the **Alert Settings** section on the dashboard:

- **Low Threshold** -- alert when temperature drops below this value
- **High Threshold** -- alert when temperature rises above this value
- **Cooldown** -- minimum time between alerts (default 30 minutes)

### Verifying the Bot

On first start, the bot creates an unverified Matrix session. To verify it:

1. Open Element (or another Matrix client) and find the bot's session
2. Start verification -- the bot will auto-accept and auto-confirm the emoji match
3. Confirm the emojis on your side

The verified session persists across restarts.

## API Endpoints

| Endpoint | Description |
|---|---|
| `GET /` | Dashboard page |
| `GET /api/current` | Latest reading as JSON |
| `GET /api/readings?hours=24` | Readings for the last N hours (`24` or `168`) |
| `GET /api/readings/day?date=YYYY-MM-DD` | All readings for a specific date |
| `GET /api/calendar?year=YYYY&month=MM` | Days with data for a given month |
| `GET /api/settings` | Current alert thresholds and cooldown |
| `POST /api/settings` | Update alert thresholds and cooldown |

## Tech Stack

- **Go** -- backend and polling
- **SQLite** (via modernc.org/sqlite) -- storage
- **mautrix-go** -- Matrix client with E2EE
- **HTMX** -- live partial updates
- **Chart.js** -- temperature charts
- **Tailwind CSS** -- styling
- **Docker** (distroless) -- deployment
