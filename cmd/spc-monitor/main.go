package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bladesdaniel-bot/spc-monitor/internal/spc"
	"github.com/bladesdaniel-bot/spc-monitor/internal/store"
)

type server struct {
	store *store.Store
}

type measurementRequest struct {
	Station        string    `json:"station"`
	Characteristic string    `json:"characteristic"`
	Value          *float64  `json:"value"`
	Timestamp      time.Time `json:"timestamp"`
}

func main() {
	srv := &server{store: store.New()}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /measurements", srv.addMeasurement)
	mux.HandleFunc("GET /measurements", srv.listMeasurements)
	mux.HandleFunc("GET /limits", srv.getLimits)

	addr := ":8090"
	log.Printf("spc-monitor listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "spc-monitor",
	})
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

func (s *server) getLimits(w http.ResponseWriter, r *http.Request) {
	station := r.URL.Query().Get("station")
	characteristic := r.URL.Query().Get("characteristic")
	if station == "" || characteristic == "" {
		writeError(w, http.StatusBadRequest, "station and characteristic query params are required")
		return
	}

	data := s.store.Get(station, characteristic)
	values := make([]float64, len(data))
	for i, m := range data {
		values[i] = m.Value
	}

	limits, err := spc.ComputeLimits(values)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, limits)
}
