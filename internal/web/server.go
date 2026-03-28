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
	db   *db.DB
	tmpl *template.Template
	mux  *http.ServeMux
}

func NewServer(database *db.DB) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	s := &Server{db: database, tmpl: tmpl, mux: http.NewServeMux()}
	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/api/current", s.handleCurrent)
	s.mux.HandleFunc("/api/readings", s.handleReadings)
	s.mux.HandleFunc("/partial/current", s.handleCurrentPartial)
	s.mux.HandleFunc("/api/readings/day", s.handleDayReadings)
	s.mux.HandleFunc("/api/calendar", s.handleCalendar)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
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

	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", latest); err != nil {
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
