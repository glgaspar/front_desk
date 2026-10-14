package apps

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ContainerStats struct {
	Id         string  `json:"id"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	MemPercent float64 `json:"memPercent"`
	MemUsage   string  `json:"memUsage"`
	NetIO      string  `json:"netIO"`
	BlockIO    string  `json:"blockIO"`
	PIDs       string  `json:"pids"`
}

var statsCache []ContainerStats
var statsCacheTime time.Time
var statsLock sync.Mutex

// docker stats takes like 2 seconds, so everyone polling shares the same answer for a bit
func GetStats() ([]ContainerStats, error) {
	statsLock.Lock()
	defer statsLock.Unlock()

	if time.Since(statsCacheTime) < 5*time.Second {
		return statsCache, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	output, err := runDocker(ctx, "", "stats", "--no-stream", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}

	stats := []ContainerStats{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		var raw struct {
			ID       string `json:"ID"`
			Name     string `json:"Name"`
			CPUPerc  string `json:"CPUPerc"`
			MemPerc  string `json:"MemPerc"`
			MemUsage string `json:"MemUsage"`
			NetIO    string `json:"NetIO"`
			BlockIO  string `json:"BlockIO"`
			PIDs     string `json:"PIDs"`
		}
		err := json.Unmarshal([]byte(line), &raw)
		if err != nil {
			continue
		}
		stats = append(stats, ContainerStats{
			Id:         raw.ID,
			Name:       raw.Name,
			CPUPercent: percent(raw.CPUPerc),
			MemPercent: percent(raw.MemPerc),
			MemUsage:   raw.MemUsage,
			NetIO:      raw.NetIO,
			BlockIO:    raw.BlockIO,
			PIDs:       raw.PIDs,
		})
	}

	statsCache = stats
	statsCacheTime = time.Now()
	return stats, nil
}

func percent(value string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "%"), 64)
	return f
}
