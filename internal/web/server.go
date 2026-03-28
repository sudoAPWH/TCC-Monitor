package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"tcc-monitor/internal/db"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	db            *db.DB
	tmpl          *template.Template
	mux           *http.ServeMux
	appTitle      string
	matrixEnabled bool
}

func NewServer(database *db.DB, appTitle string, matrixEnabled bool) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	s := &Server{db: database, tmpl: tmpl, mux: http.NewServeMux(), appTitle: appTitle, matrixEnabled: matrixEnabled}
	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/api/current", s.handleCurrent)
	s.mux.HandleFunc("/api/readings", s.handleReadings)
	s.mux.HandleFunc("/partial/current", s.handleCurrentPartial)
	s.mux.HandleFunc("/api/readings/day", s.handleDayReadings)
	s.mux.HandleFunc("/api/calendar", s.handleCalendar)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type dashboardData struct {
	Title           string
	Temperature     float64
	Setpoint        float64
	ThresholdLow    float64
	ThresholdHigh   float64
	CooldownMinutes int
	MatrixEnabled   bool
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	latest, err := s.db.Latest()
	if err != nil {
		latest = &db.Reading{}
	}

	low, high, _ := s.db.GetThresholds()

	data := dashboardData{
		Title:           s.appTitle,
		Temperature:     latest.Temperature,
		Setpoint:        latest.Setpoint,
		ThresholdLow:    low,
		ThresholdHigh:   high,
		CooldownMinutes: s.db.GetCooldownMinutes(),
		MatrixEnabled:   s.matrixEnabled,
	}

	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		log.Printf("web: template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleCurrentPartial returns just the hero stat card HTML for HTMX polling.
func (s *Server) handleCurrentPartial(w http.ResponseWriter, r *http.Request) {
	latest, err := s.db.Latest()
	if err != nil {
		latest = &db.Reading{}
	}

	if err := s.tmpl.ExecuteTemplate(w, "current_partial.html", latest); err != nil {
		log.Printf("web: partial template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

type readingsResponse struct {
	Labels    []string  `json:"labels"`
	Temps     []float64 `json:"temps"`
	Setpoints []float64 `json:"setpoints"`
}

func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	latest, err := s.db.Latest()
	if err != nil {
		http.Error(w, "no data", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(latest)
}

func (s *Server) handleReadings(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr == "168" {
		hours = 168
	}

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	readings, err := s.db.ReadingsSince(since)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	resp := readingsResponse{
		Labels:    make([]string, len(readings)),
		Temps:     make([]float64, len(readings)),
		Setpoints: make([]float64, len(readings)),
	}

	for i, r := range readings {
		resp.Labels[i] = r.Timestamp.Local().Format("Jan 2 3:04 PM")
		resp.Temps[i] = r.Temperature
		resp.Setpoints[i] = r.Setpoint
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDayReadings(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "date parameter required (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	readings, err := s.db.ReadingsForDay(date)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	resp := readingsResponse{
		Labels:    make([]string, len(readings)),
		Temps:     make([]float64, len(readings)),
		Setpoints: make([]float64, len(readings)),
	}
	for i, r := range readings {
		resp.Labels[i] = r.Timestamp.Local().Format("3:04 PM")
		resp.Temps[i] = r.Temperature
		resp.Setpoints[i] = r.Setpoint
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if v := r.URL.Query().Get("year"); v != "" {
		if y, err := strconv.Atoi(v); err == nil {
			year = y
		}
	}
	if v := r.URL.Query().Get("month"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			month = m
		}
	}

	days, err := s.db.DaysWithData(year, month)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"year":  year,
		"month": month,
		"days":  days,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		low, high, _ := s.db.GetThresholds()
		roomID, _ := s.db.GetSetting("matrix_room_id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"threshold_low":    low,
			"threshold_high":   high,
			"cooldown_minutes": s.db.GetCooldownMinutes(),
			"matrix_room_id":  roomID,
			"matrix_enabled":  s.matrixEnabled,
		})

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if v := r.FormValue("threshold_low"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				s.db.SetSetting("threshold_low", strconv.FormatFloat(f, 'f', 1, 64))
			}
		}
		if v := r.FormValue("threshold_high"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				s.db.SetSetting("threshold_high", strconv.FormatFloat(f, 'f', 1, 64))
			}
		}
		if v := r.FormValue("cooldown_minutes"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				s.db.SetSetting("cooldown_minutes", strconv.Itoa(n))
			}
		}
		if v := r.FormValue("matrix_room_id"); v != "" {
			s.db.SetSetting("matrix_room_id", v)
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<span class="text-green-600 font-medium">Settings saved!</span>`))

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
