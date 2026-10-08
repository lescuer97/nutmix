package templates

type LDKResourceSnapshot struct {
	CPUAvailable    string
	MemoryAvailable string
	DiskAvailable   string
}

func DefaultLDKResourceSnapshot() LDKResourceSnapshot {
	return LDKResourceSnapshot{
		CPUAvailable:    "Unavailable",
		MemoryAvailable: "Unavailable",
		DiskAvailable:   "Unavailable",
	}
}
