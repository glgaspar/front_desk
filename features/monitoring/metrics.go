package monitoring

import (
	"github.com/glgaspar/front_desk/connection"
	"github.com/glgaspar/front_desk/features/system"
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

	// can fail on some setups, not worth losing the sample over it
	var load1 *float64
	if avg, err := load.Avg(); err == nil {
		load1 = &avg.Load1
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}

	_, err = conn.Exec(`
	insert into frontdesk.host_metrics (cpu_percent, mem_percent, load1)
	values ($1, $2, $3)`, *cpu, *mem, load1)
	if err != nil {
		return err
	}

	for _, d := range system.GetDisksUsage() {
		_, err = conn.Exec(`
		insert into frontdesk.host_disk_metrics (path, used_percent, used_bytes, total_bytes)
		values ($1, $2, $3, $4)`, d.Path, d.UsedPercent, d.Used, d.Total)
		if err != nil {
			return err
		}
	}
	return nil
}
