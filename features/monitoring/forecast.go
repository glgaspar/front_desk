package monitoring

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/glgaspar/front_desk/features/activity"
	"github.com/glgaspar/front_desk/features/apps"
)

type DiskForecast struct {
	Path         string   `json:"path"`
	UsedPercent  float64  `json:"usedPercent"`
	UsedBytes    int64    `json:"usedBytes"`
	TotalBytes   int64    `json:"totalBytes"`
	GrowthPerDay float64  `json:"growthPerDay"` // bytes, negative when it is shrinking
	DaysToFull   *float64 `json:"daysToFull"`   // nil when it is not growing or there is too little data
}

// a straight line over the last 7 days. torrents finishing and tdarr shrinking files make it
// jumpy, but its good enough to tell "weeks" from "months"
func GetDiskForecast() ([]DiskForecast, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select
		path,
		coalesce(regr_slope(used_bytes, extract(epoch from sampled_at)), 0),
		(array_agg(used_percent order by sampled_at desc))[1],
		(array_agg(used_bytes order by sampled_at desc))[1],
		(array_agg(total_bytes order by sampled_at desc))[1],
		extract(epoch from max(sampled_at) - min(sampled_at))
	from frontdesk.host_disk_metrics
	where sampled_at > now() - interval '7 days'
	group by path
	order by path`
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []DiskForecast{}
	for rows.Next() {
		var f DiskForecast
		var slope, span float64
		err = rows.Scan(&f.Path, &slope, &f.UsedPercent, &f.UsedBytes, &f.TotalBytes, &span)
		if err != nil {
			return nil, err
		}
		f.GrowthPerDay = slope * 86400

		// less than 6 hours of data is just noise
		if span >= 6*3600 && f.GrowthPerDay > 0 {
			days := float64(f.TotalBytes-f.UsedBytes) / f.GrowthPerDay
			days = math.Round(days*10) / 10
			f.DaysToFull = &days
		}
		list = append(list, f)
	}
	return list, nil
}

func humanBytes(b float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for math.Abs(b) >= 1024 && i < len(units)-1 {
		b /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", b, units[i])
}

// everything worth telling about the last day, written as plain lines for ollama to read
func DailyFacts() (string, error) {
	conn, err := connection.Db()
	if err != nil {
		return "", err
	}
	var facts []string

	// one check a minute, so failed checks are roughly minutes down
	rows, err := conn.Query(`
	select coalesce(max(app_name), app_key), count(*) filter (where not up), count(*)
	from frontdesk.health_checks
	where checked_at > now() - interval '24 hours'
	group by app_key
	having count(*) filter (where not up) > 0
	order by 2 desc`)
	if err != nil {
		return "", err
	}
	down := []string{}
	total := 0
	for rows.Next() {
		var name string
		var failed, checks int
		rows.Scan(&name, &failed, &checks)
		down = append(down, fmt.Sprintf("%s failed %d of %d checks", name, failed, checks))
		total++
	}
	rows.Close()
	if total == 0 {
		facts = append(facts, "Every monitored app passed every health check.")
	} else {
		facts = append(facts, fmt.Sprintf("%d apps had failed health checks (one check per minute): %s.", total, strings.Join(down, "; ")))
	}

	var stillDown []string
	rows, err = conn.Query(`select app_key from frontdesk.monitor_state where not up`)
	if err == nil {
		for rows.Next() {
			var key string
			rows.Scan(&key)
			stillDown = append(stillDown, key)
		}
		rows.Close()
	}
	if len(stillDown) > 0 {
		facts = append(facts, "Still down right now: "+strings.Join(stillDown, ", ")+".")
	}

	var cpuAvg, cpuMax, memAvg, memMax float64
	err = conn.QueryRow(`
	select coalesce(avg(cpu_percent), 0), coalesce(max(cpu_percent), 0), coalesce(avg(mem_percent), 0), coalesce(max(mem_percent), 0)
	from frontdesk.host_metrics where sampled_at > now() - interval '24 hours'`).Scan(&cpuAvg, &cpuMax, &memAvg, &memMax)
	if err == nil {
		facts = append(facts, fmt.Sprintf("Host CPU averaged %.0f%% (peak %.0f%%), memory averaged %.0f%% (peak %.0f%%).", cpuAvg, cpuMax, memAvg, memMax))
	}

	disks, err := GetDiskForecast()
	if err == nil {
		for _, d := range disks {
			line := fmt.Sprintf("Disk %s is %.0f%% full, changing %s per day", d.Path, d.UsedPercent, humanBytes(d.GrowthPerDay))
			if d.DaysToFull != nil {
				line += fmt.Sprintf(", full in about %.0f days at this rate", *d.DaysToFull)
			}
			facts = append(facts, line+".")
		}
	}

	appList, err := new(apps.App).GetList()
	if err == nil {
		var broken []string
		for _, a := range appList {
			if a.Broken() {
				broken = append(broken, fmt.Sprintf("%s (%s, exit code %d)", appName(a), a.State.Status, a.State.ExitCode))
			}
		}
		if len(broken) > 0 {
			facts = append(facts, "Containers not running because they failed: "+strings.Join(broken, ", ")+".")
		}
	}

	updates, err := apps.GetAllUpdates(false)
	if err == nil {
		var refs []string
		for ref, u := range updates {
			if u.Status == "update_available" {
				refs = append(refs, ref)
			}
		}
		sort.Strings(refs)
		if len(refs) > 0 {
			facts = append(facts, fmt.Sprintf("%d images have updates available: %s.", len(refs), strings.Join(refs, ", ")))
		}
	}

	entries, err := activity.List(activity.Filter{Limit: 200})
	if err == nil {
		counts := map[string]int{}
		for _, e := range entries {
			// only what changed something, opening pages and reading logs is noise here
			if e.CreatedAt.After(time.Now().Add(-24*time.Hour)) && !noise(e.Action) {
				counts[e.Action]++
			}
		}
		if len(counts) > 0 {
			var done []string
			for action, n := range counts {
				done = append(done, fmt.Sprintf("%s x%d", action, n))
			}
			sort.Strings(done)
			facts = append(facts, "Changes people made: "+strings.Join(done, ", ")+".")
		}
	}

	return strings.Join(facts, "\n"), nil
}

func noise(action string) bool {
	for _, prefix := range []string{"opened", "watched", "checked for updates", "registered widget", "GET "} {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return false
}

func appName(a apps.App) string {
	if a.Name != "" {
		return a.Name
	}
	return a.ImageRef
}
