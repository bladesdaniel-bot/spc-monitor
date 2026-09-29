package store

import (
	"sync"
	"time"
)

type Measurement struct {
	Station        string    `json:"station"`
	Characteristic string    `json:"characteristic"`
	Value          float64   `json:"value"`
	Timestamp      time.Time `json:"timestamp"`
}

type Store struct {
	mu     sync.RWMutex
	series map[string][]Measurement
}

func New() *Store {
	return &Store{series: make(map[string][]Measurement)}
}

func key(station, characteristic string) string {
	return station + "|" + characteristic
}

func (s *Store) Add(m Measurement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(m.Station, m.Characteristic)
	s.series[k] = append(s.series[k], m)
}

func (s *Store) Get(station, characteristic string) []Measurement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.series[key(station, characteristic)]
	out := make([]Measurement, len(src))
	copy(out, src)
	return out
}
