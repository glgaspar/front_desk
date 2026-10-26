package system

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type ProcUsage struct {
	PID        int32    `json:"pid"`
	Name       string   `json:"name"`
	CPUPercent *float64 `json:"cpuPercent,omitempty"`
	MemPercent *float32 `json:"memPercent,omitempty"`
}

type DiskUsage struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"usedPercent"`
}

type SystemUsage struct {
	CPUPercent    float64     `json:"cpuPercent"`
	MemoryPercent float64     `json:"memoryPercent"`
	Disks         []DiskUsage `json:"disks"`
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

	s.Disks = GetDisksUsage()

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

// HOST_DISKS is a comma separated list of paths, one per disk. they need to be mounted in the container
func DiskPaths() []string {
	value := os.Getenv("HOST_DISKS")
	if value == "" {
		value = os.Getenv("HOST_DISK_PATH")
	}
	var paths []string
	for _, path := range strings.Split(value, ",") {
		if path = strings.TrimSpace(path); path != "" {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		paths = []string{"/"}
	}
	return paths
}

// a disk that isnt mounted just doesnt show up, no reason to lose the rest over it
func GetDisksUsage() []DiskUsage {
	disks := []DiskUsage{}
	for _, path := range DiskPaths() {
		usage, err := disk.Usage(path)
		if err != nil {
			continue
		}
		disks = append(disks, DiskUsage{Path: path, Total: usage.Total, Used: usage.Used, UsedPercent: usage.UsedPercent})
	}
	return disks
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
