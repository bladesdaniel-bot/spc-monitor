package store

import (
	"sort"
	"sync"
	"time"
)

type Measurement struct {
	Station        string    `json:"station"`
	Characteristic string    `json:"characteristic"`
	Value          float64   `json:"value"`
	Timestamp      time.Time `json:"timestamp"`
}

type SeriesInfo struct {
	Station        string `json:"station"`
	Characteristic string `json:"characteristic"`
	Count          int    `json:"count"`
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

func (s *Store) Series() []SeriesInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SeriesInfo, 0, len(s.series))
	for _, ms := range s.series {
		if len(ms) == 0 {
			continue
		}
		out = append(out, SeriesInfo{
			Station:        ms[0].Station,
			Characteristic: ms[0].Characteristic,
			Count:          len(ms),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Station != out[j].Station {
			return out[i].Station < out[j].Station
		}
		return out[i].Characteristic < out[j].Characteristic
	})
	return out
}
