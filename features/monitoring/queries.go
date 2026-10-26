package monitoring

import (
	"fmt"
	"time"

	"github.com/glgaspar/front_desk/connection"
)

type rangeInfo struct {
	since  time.Duration
	bucket int // seconds
}

var ranges = map[string]rangeInfo{
	"1h":  {time.Hour, 60},
	"24h": {24 * time.Hour, 15 * 60},
	"7d":  {7 * 24 * time.Hour, 60 * 60},
	"30d": {30 * 24 * time.Hour, 6 * 60 * 60},
}

func getRange(r string) (rangeInfo, error) {
	if r == "" {
		r = "24h"
	}
	info, ok := ranges[r]
	if !ok {
		return info, fmt.Errorf("range must be 1h, 24h, 7d or 30d")
	}
	return info, nil
}

type Uptime struct {
	AppKey        string    `json:"appKey"`
	AppName       string    `json:"appName"`
	UptimePercent float64   `json:"uptimePercent"`
	Up            bool      `json:"up"`
	ChangedAt     time.Time `json:"changedAt"`
	AvgLatencyMs  *float64  `json:"avgLatencyMs"`
	Checks        int       `json:"checks"`
}

func GetUptime(r string) ([]Uptime, error) {
	info, err := getRange(r)
	if err != nil {
		return nil, err
	}
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select
		h.app_key,
		max(h.app_name),
		100.0 * count(*) filter (where h.up) / count(*),
		coalesce(bool_and(s.up), true),
		coalesce(max(s.changed_at), now()),
		avg(h.latency_ms),
		count(*)
	from frontdesk.health_checks h
	left join frontdesk.monitor_state s on s.app_key = h.app_key
	where h.checked_at > $1
	group by h.app_key
	order by max(h.app_name)`
	rows, err := conn.Query(query, time.Now().Add(-info.since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Uptime{}
	for rows.Next() {
		var u Uptime
		err = rows.Scan(&u.AppKey, &u.AppName, &u.UptimePercent, &u.Up, &u.ChangedAt, &u.AvgLatencyMs, &u.Checks)
		if err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, nil
}

type UptimePoint struct {
	Time          time.Time `json:"time"`
	UptimePercent float64   `json:"uptimePercent"`
	AvgLatencyMs  *float64  `json:"avgLatencyMs"`
}

func GetUptimeHistory(appKey string, r string) ([]UptimePoint, error) {
	info, err := getRange(r)
	if err != nil {
		return nil, err
	}
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select
		to_timestamp(floor(extract(epoch from checked_at) / $3) * $3) as bucket,
		100.0 * count(*) filter (where up) / count(*),
		avg(latency_ms)
	from frontdesk.health_checks
	where app_key = $1 and checked_at > $2
	group by bucket
	order by bucket`
	rows, err := conn.Query(query, appKey, time.Now().Add(-info.since), info.bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []UptimePoint{}
	for rows.Next() {
		var p UptimePoint
		err = rows.Scan(&p.Time, &p.UptimePercent, &p.AvgLatencyMs)
		if err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

type MetricPoint struct {
	Time       time.Time          `json:"time"`
	CPUPercent float64            `json:"cpuPercent"`
	MemPercent float64            `json:"memPercent"`
	Load1      *float64           `json:"load1"`
	Disks      map[string]float64 `json:"disks"` // path -> used percent
}

func GetMetricsHistory(r string) ([]MetricPoint, error) {
	info, err := getRange(r)
	if err != nil {
		return nil, err
	}
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select
		to_timestamp(floor(extract(epoch from sampled_at) / $2) * $2) as bucket,
		avg(cpu_percent),
		avg(mem_percent),
		avg(load1)
	from frontdesk.host_metrics
	where sampled_at > $1
	group by bucket
	order by bucket`
	rows, err := conn.Query(query, time.Now().Add(-info.since), info.bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []MetricPoint{}
	byTime := map[int64]int{}
	for rows.Next() {
		p := MetricPoint{Disks: map[string]float64{}}
		err = rows.Scan(&p.Time, &p.CPUPercent, &p.MemPercent, &p.Load1)
		if err != nil {
			return nil, err
		}
		byTime[p.Time.Unix()] = len(points)
		points = append(points, p)
	}

	diskQuery := `
	select
		to_timestamp(floor(extract(epoch from sampled_at) / $2) * $2) as bucket,
		path,
		avg(used_percent)
	from frontdesk.host_disk_metrics
	where sampled_at > $1
	group by bucket, path`
	diskRows, err := conn.Query(diskQuery, time.Now().Add(-info.since), info.bucket)
	if err != nil {
		return nil, err
	}
	defer diskRows.Close()

	for diskRows.Next() {
		var bucket time.Time
		var path string
		var percent float64
		err = diskRows.Scan(&bucket, &path, &percent)
		if err != nil {
			return nil, err
		}
		// both are sampled together, a disk bucket without a host bucket is not worth a hole in the chart
		if i, ok := byTime[bucket.Unix()]; ok {
			points[i].Disks[path] = percent
		}
	}
	return points, nil
}
