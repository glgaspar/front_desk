package monitoring

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"
)

func envDuration(name string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(os.Getenv(name))
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func envInt(name string, def int) int {
	i, err := strconv.Atoi(os.Getenv(name))
	if err != nil || i <= 0 {
		return def
	}
	return i
}

// first thing that runs on its own in here. checks the apps, saves the host usage and cleans old stuff
func Start(ctx context.Context) {
	go every(ctx, envDuration("MONITOR_INTERVAL", time.Minute), "app checks", RunChecks)
	go every(ctx, envDuration("METRICS_INTERVAL", time.Minute), "host metrics", SaveMetrics)
	go every(ctx, time.Hour, "cleanup", Cleanup)
}

func every(ctx context.Context, interval time.Duration, name string, job func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	run := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("monitoring %s panicked: %v", name, r)
			}
		}()
		err := job()
		if err != nil {
			log.Printf("monitoring %s failed: %v", name, err)
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
