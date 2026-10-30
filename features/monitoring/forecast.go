package monitoring

import (
	"math"

	"github.com/glgaspar/front_desk/connection"
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
