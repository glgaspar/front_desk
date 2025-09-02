package system

import (
	"sort"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type ProcUsage struct {
	PID        int32
	Name       string
	CPUPercent float64
	MemPercent float32
}

type SystemUsage struct {
	CPUPercent    float64       `json:"cpuPercent"`
	MemoryPercent float64       `json:"memoryPercent"`
	ProcUsage     []ProcUsage   `json:"procUsage"`
}

func (s *SystemUsage) GetSystemUsage() error {
	cpuPercent, err := GetCPUUsage()
	if err != nil {
		return err
	}
	s.CPUPercent = *cpuPercent

	memoryPercent, err := GetMemoryUsage()
	if err != nil {
		return err
	}
	s.MemoryPercent = *memoryPercent

	processesUsage, err := GetProcessesUsage()
	if err != nil {
		return err
	}
	s.ProcUsage = processesUsage


	return nil
}

func GetCPUUsage() (*float64, error) {
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}
	return &percentages[0], nil
}

func GetMemoryUsage() (*float64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	return &vmStat.UsedPercent, nil
}

func GetProcessesUsage() ([]ProcUsage, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}


	// TODO: split mem and cpu arrays because of ordering
	var usages []ProcUsage
	for _, p := range procs {
		cpuPercent, err1 := p.CPUPercent()
		memPercent, err2 := p.MemoryPercent()
		name, _ := p.Name()
		if err1 == nil && err2 == nil {
			usages = append(usages, ProcUsage{
				PID:        p.Pid,
				Name:       name,
				CPUPercent: cpuPercent,
				MemPercent: memPercent,
			})
		}
	}

	sort.Slice(usages, func(i, j int) bool {
		return usages[i].CPUPercent > usages[j].CPUPercent
	})

	//if you are more than 15 processes deep, you should go check the server properly
	if len(usages) > 15 {
		usages = usages[:15]
	}

	return usages, nil
}
