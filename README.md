# TCC Monitor

A lightweight, self-hosted dashboard for monitoring Honeywell Total Connect Comfort (TCC) thermostats. Polls your thermostat on a configurable interval and stores temperature readings in a local SQLite database, then serves a clean web dashboard with live stats and historical charts.

## Features

- **Live polling** of temperature and heat setpoint from the TCC API
- **Real-time dashboard** with current stats that auto-refresh via HTMX
- **Temperature trend charts** for the last 24 hours or 7 days (Chart.js)
- **Calendar history view** -- click any day to see that day's readings
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

## API Endpoints

| Endpoint | Description |
|---|---|
| `GET /` | Dashboard page |
| `GET /api/current` | Latest reading as JSON |
| `GET /api/readings?hours=24` | Readings for the last N hours (`24` or `168`) |
| `GET /api/readings/day?date=YYYY-MM-DD` | All readings for a specific date |
| `GET /api/calendar?year=YYYY&month=MM` | Days with data for a given month |

## Tech Stack

- **Go** -- backend and polling
- **SQLite** (via modernc.org/sqlite) -- storage
- **HTMX** -- live partial updates
- **Chart.js** -- temperature charts
- **Tailwind CSS** -- styling
- **Docker** (distroless) -- deployment
