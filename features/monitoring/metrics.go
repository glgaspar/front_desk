package monitoring

import (
	"os"

	"github.com/glgaspar/front_desk/connection"
	"github.com/glgaspar/front_desk/features/system"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
)

func SaveMetrics() error {
	cpu, err := system.GetCPUUsage()
	if err != nil {
		return err
	}
	mem, err := system.GetMemoryUsage()
	if err != nil {
		return err
	}

	// these two can fail on some setups, not worth losing the sample over it
	var diskPercent *float64
	path := os.Getenv("HOST_DISK_PATH")
	if path == "" {
		path = "/"
	}
	if usage, err := disk.Usage(path); err == nil {
		diskPercent = &usage.UsedPercent
	}

	var load1 *float64
	if avg, err := load.Avg(); err == nil {
		load1 = &avg.Load1
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}

	_, err = conn.Exec(`
	insert into frontdesk.host_metrics (cpu_percent, mem_percent, disk_percent, load1)
	values ($1, $2, $3, $4)`, *cpu, *mem, diskPercent, load1)
	return err
}
