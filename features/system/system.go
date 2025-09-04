package system

import (
	"sort"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type ProcUsage struct {
	PID        int32    `json:"pid"`
	Name       string   `json:"name"`
	CPUPercent *float64 `json:"cpuPercent,omitempty"`
	MemPercent *float32 `json:"memPercent,omitempty"`
}

type SystemUsage struct {
	CPUPercent    float64     `json:"cpuPercent"`
	MemoryPercent float64     `json:"memoryPercent"`
	ProcMem       []ProcUsage `json:"procMem"`
	ProcCPU       []ProcUsage `json:"procCPU"`
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

	CPUUsages, memUsages, err := GetProcessesUsage()
	if err != nil {
		return err
	}

	s.ProcMem = memUsages
	s.ProcCPU = CPUUsages

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

func GetProcessesUsage() ([]ProcUsage, []ProcUsage, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, nil, err
	}

	var CPUUsages []ProcUsage
	var memUsages []ProcUsage
	for _, p := range procs {
		cpuPercent, err1 := p.CPUPercent()
		memPercent, err2 := p.MemoryPercent()
		name, _ := p.Name()
		if err1 == nil && err2 == nil {
			CPUUsages = append(CPUUsages, ProcUsage{
				PID:        p.Pid,
				Name:       name,
				CPUPercent: &cpuPercent,
			})
			memUsages = append(memUsages, ProcUsage{
				PID:        p.Pid,
				Name:       name,
				MemPercent: &memPercent,
			})
		}
	}

	sort.Slice(CPUUsages, func(i, j int) bool {
		return *CPUUsages[i].CPUPercent > *CPUUsages[j].CPUPercent
	})
	sort.Slice(memUsages, func(i, j int) bool {
		return *memUsages[i].MemPercent > *memUsages[j].MemPercent
	})

	//if you are more than 15 processes deep, you should go check the server properly
	if len(memUsages) > 15 {
		memUsages = memUsages[:15]
	}
	if len(CPUUsages) > 15 {
		CPUUsages = CPUUsages[:15]
	}

	return CPUUsages, memUsages, nil
}
