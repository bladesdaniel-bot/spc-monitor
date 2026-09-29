package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bladesdaniel-bot/spc-monitor/internal/spc"
	"github.com/bladesdaniel-bot/spc-monitor/internal/store"
)

//go:embed web/Dashboard.html
var dashboardHTML []byte

const defaultBaseline = 20

type server struct {
	store *store.Store
	mu    sync.RWMutex
	specs map[string]spc.Spec
}

type measurementRequest struct {
	Station        string    `json:"station"`
	Characteristic string    `json:"characteristic"`
	Value          *float64  `json:"value"`
	Timestamp      time.Time `json:"timestamp"`
}

type specRequest struct {
	Station        string   `json:"station"`
	Characteristic string   `json:"characteristic"`
	LSL            *float64 `json:"lsl"`
	USL            *float64 `json:"usl"`
}

func main() {
	srv := &server{store: store.New(), specs: make(map[string]spc.Spec)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", serveDashboard)
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /series", srv.listSeries)
	mux.HandleFunc("POST /measurements", srv.addMeasurement)
	mux.HandleFunc("GET /measurements", srv.listMeasurements)
	mux.HandleFunc("GET /limits", srv.getLimits)
	mux.HandleFunc("GET /violations", srv.getViolations)
	mux.HandleFunc("PUT /specs", srv.setSpec)

	addr := ":8090"
	log.Printf("spc-monitor listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, withCORS(mux)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func specKey(station, characteristic string) string {
	return station + "|" + characteristic
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(dashboardHTML)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "spc-monitor",
	})
}

func (s *server) listSeries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Series())
}

func (s *server) addMeasurement(w http.ResponseWriter, r *http.Request) {
	var req measurementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Station == "" || req.Characteristic == "" {
		writeError(w, http.StatusBadRequest, "station and characteristic are required")
		return
	}
	if req.Value == nil {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now().UTC()
	}

	m := store.Measurement{
		Station:        req.Station,
		Characteristic: req.Characteristic,
		Value:          *req.Value,
		Timestamp:      req.Timestamp,
	}
	s.store.Add(m)
	writeJSON(w, http.StatusCreated, m)
}

func (s *server) listMeasurements(w http.ResponseWriter, r *http.Request) {
	station := r.URL.Query().Get("station")
	characteristic := r.URL.Query().Get("characteristic")
	if station == "" || characteristic == "" {
		writeError(w, http.StatusBadRequest, "station and characteristic query params are required")
		return
	}
	writeJSON(w, http.StatusOK, s.store.Get(station, characteristic))
}

func (s *server) setSpec(w http.ResponseWriter, r *http.Request) {
	var req specRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Station == "" || req.Characteristic == "" || req.LSL == nil || req.USL == nil {
		writeError(w, http.StatusBadRequest, "station, characteristic, lsl and usl are required")
		return
	}
	spec := spc.Spec{LSL: *req.LSL, USL: *req.USL}
	if spec.USL <= spec.LSL {
		writeError(w, http.StatusBadRequest, spc.ErrBadSpec.Error())
		return
	}

	s.mu.Lock()
	s.specs[specKey(req.Station, req.Characteristic)] = spec
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, spec)
}

func (s *server) spec(station, characteristic string) (spc.Spec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sp, ok := s.specs[specKey(station, characteristic)]
	return sp, ok
}

// baseline loads a series and computes limits from its first N readings.
// It writes an error response and returns ok=false if anything is missing.
func (s *server) baseline(w http.ResponseWriter, r *http.Request) ([]float64, spc.Limits, bool) {
	station := r.URL.Query().Get("station")
	characteristic := r.URL.Query().Get("characteristic")
	if station == "" || characteristic == "" {
		writeError(w, http.StatusBadRequest, "station and characteristic query params are required")
		return nil, spc.Limits{}, false
	}

	n := defaultBaseline
	if q := r.URL.Query().Get("baseline"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || v < 2 {
			writeError(w, http.StatusBadRequest, "baseline must be a whole number of at least 2")
			return nil, spc.Limits{}, false
		}
		n = v
	}

	data := s.store.Get(station, characteristic)
	if len(data) < n {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("need %d readings for a baseline, have %d", n, len(data)))
		return nil, spc.Limits{}, false
	}

	values := make([]float64, len(data))
	for i, m := range data {
		values[i] = m.Value
	}

	limits, err := spc.ComputeLimits(values[:n])
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return nil, spc.Limits{}, false
	}
	return values, limits, true
}

func (s *server) getLimits(w http.ResponseWriter, r *http.Request) {
	_, limits, ok := s.baseline(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, limits)
}

func (s *server) getViolations(w http.ResponseWriter, r *http.Request) {
	values, limits, ok := s.baseline(w, r)
	if !ok {
		return
	}
	resp := map[string]any{
		"limits":       limits,
		"violations":   spc.Check(values, limits),
		"mrViolations": spc.CheckMR(values, limits),
	}

	station := r.URL.Query().Get("station")
	characteristic := r.URL.Query().Get("characteristic")
	if sp, ok := s.spec(station, characteristic); ok {
		resp["spec"] = sp
		if c, err := spc.ComputeCapability(values, sp); err == nil {
			resp["capability"] = c
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// withCORS lets browser pages (like the plant simulator) send readings to the monitor.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			h.Set("Access-Control-Allow-Private-Network", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
