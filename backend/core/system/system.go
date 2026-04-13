package system

import (
	"net"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

var StartTime time.Time
var Version = "v1.0.0-dev"

func init() {
	StartTime = time.Now()
}

type SystemStats struct {
	CPU           float64 `json:"cpu"`
	MemoryTotal   uint64  `json:"memory_total"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskTotal     uint64  `json:"disk_total"`
	DiskUsed      uint64  `json:"disk_used"`
	DiskPercent   float64 `json:"disk_percent"`
	ServiceCPU    float64 `json:"service_cpu"`
	ServiceMemory uint64  `json:"service_memory"`
	Uptime        uint64  `json:"uptime"`
	IP            string  `json:"ip"`
	OS            string  `json:"os"`
	Arch          string  `json:"arch"`
	Version       string  `json:"version"`
	PID           int     `json:"pid"`
	NumGoroutine  int     `json:"num_goroutine"`
	NumGC         uint32  `json:"num_gc"`
	GoVersion     string  `json:"go_version"`
}

func GetStats() (*SystemStats, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	c, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}

	// Disk usage: sum of all physical partitions or just use "/" for root/C:
	// To be cross-platform and simple for now, let's use the partition where the binary runs, or better, iterate.
	// Simple approach: Total disk space of the system (sum of all physical fixed drives)
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var validParts []disk.PartitionStat
	for _, p := range parts {
		if p.Fstype == "" { // Filter out empty fstype
			continue
		}
		// You might want to filter by "fixed" drives only depending on requirement
		validParts = append(validParts, p)
	}

	// For simplicity in this dashboard, let's just show the Usage of the "Root" or "System" partition.
	// On Windows, it's often C:, on Linux /.
	// A more comprehensive way is to sum up, but "Used Percent" is tricky for sum.
	// Let's stick to the path "." (Current working directory's partition) which is relevant to the app.
	d, err := disk.Usage(".")
	if err != nil {
		// Fallback to root if . fails
		d, err = disk.Usage("/")
		if err != nil && len(validParts) > 0 {
			// Fallback to first partition
			d, err = disk.Usage(validParts[0].Mountpoint)
		}
	}

	if err != nil {
		return nil, err
	}

	pid := os.Getpid()
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}

	svcCPU, err := proc.Percent(time.Second)
	if err != nil {
		svcCPU = 0
	}

	svcMemoryInfo, err := proc.MemoryInfo()
	if err != nil {
		return nil, err
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := &SystemStats{
		CPU:           c[0],
		MemoryTotal:   v.Total,
		MemoryUsed:    v.Used,
		MemoryPercent: v.UsedPercent,
		DiskTotal:     d.Total,
		DiskUsed:      d.Used,
		DiskPercent:   d.UsedPercent,
		ServiceCPU:    svcCPU,
		ServiceMemory: svcMemoryInfo.RSS,
		Uptime:        uint64(time.Since(StartTime).Seconds()),
		IP:            GetOutboundIP(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Version:       Version,
		PID:           pid,
		NumGoroutine:  runtime.NumGoroutine(),
		NumGC:         m.NumGC,
		GoVersion:     runtime.Version(),
	}

	return stats, nil
}

// GetOutboundIP gets preferred outbound ip of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}
