package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

func main() {
	baseURL := flag.String("url", "http://localhost:8090", "spc-monitor base URL")
	station := flag.String("station", "L1-GLUE", "station name")
	characteristic := flag.String("char", "bead_width_mm", "characteristic being measured")
	target := flag.Float64("target", 4.00, "process target value")
	sigma := flag.Float64("sigma", 0.03, "normal process variation (standard deviation)")
	count := flag.Int("count", 50, "number of readings to send")
	interval := flag.Duration("interval", 200*time.Millisecond, "time between readings")
	mode := flag.String("mode", "stable", "stable, drift, shift, or spike")
	start := flag.Int("start", 25, "reading number where the fault begins")
	flag.Parse()

	switch *mode {
	case "stable", "drift", "shift", "spike":
	default:
		log.Fatalf("unknown mode %q: use stable, drift, shift, or spike", *mode)
	}

	endpoint := *baseURL + "/measurements"

	for i := 0; i < *count; i++ {
		mean := *target
		faulty := *mode != "stable" && i >= *start

		if faulty {
			switch *mode {
			case "drift":
				mean += 0.1 * *sigma * float64(i-*start+1)
			case "shift":
				mean += 1.5 * *sigma
			}
		}

		value := mean + rand.NormFloat64()*(*sigma)
		if *mode == "spike" && i == *start {
			value = *target + 4*(*sigma)
		}
		value = math.Round(value*1000) / 1000

		if err := send(endpoint, *station, *characteristic, value); err != nil {
			log.Fatalf("reading %d: %v", i, err)
		}

		marker := ""
		if faulty && (*mode != "spike" || i == *start) {
			marker = "  <- " + *mode
		}
		fmt.Printf("%3d  %.3f%s\n", i, value, marker)

		time.Sleep(*interval)
	}
}

func send(endpoint, station, characteristic string, value float64) error {
	body, err := json.Marshal(map[string]any{
		"station":        station,
		"characteristic": characteristic,
		"value":          value,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}
