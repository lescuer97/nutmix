package admin

import (
	"fmt"
	"runtime"
	"syscall"

	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func getLDKResourceSnapshot() templates.LDKResourceSnapshot {
	snapshot := templates.DefaultLDKResourceSnapshot()
	snapshot.CPUAvailable = fmt.Sprintf("%d cores", runtime.NumCPU())

	if runtime.GOOS != "linux" {
		return snapshot
	}

	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err == nil {
		totalRAMBytes := info.Totalram * uint64(info.Unit)
		snapshot.MemoryAvailable = formatGiB(totalRAMBytes)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		freeDiskBytes := stat.Bavail * uint64(stat.Bsize)
		snapshot.DiskAvailable = formatGiB(freeDiskBytes)
	}

	return snapshot
}

func formatGiB(bytes uint64) string {
	const gib = 1024 * 1024 * 1024
	if bytes == 0 {
		return "Unavailable"
	}
	return fmt.Sprintf("%.1f GiB", float64(bytes)/gib)
}
