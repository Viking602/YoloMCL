package memory

type MemoryInfo struct {
	TotalMB       int `json:"totalMB"`
	AvailableMB   int `json:"availableMB"`
	MaxMB         int `json:"maxMB"`
	RecommendedMB int `json:"recommendedMB"`
}

type Info = MemoryInfo

func SystemMemory() (MemoryInfo, error) {
	total, available, e := physicalMemory()
	if e != nil {
		return MemoryInfo{}, e
	}
	maxMB := min(131072, int(total>>20)/512*512)
	recommended := min(4096, maxMB/2/512*512)
	return MemoryInfo{TotalMB: int(total >> 20), AvailableMB: int(available >> 20), MaxMB: max(512, maxMB), RecommendedMB: max(512, recommended)}, nil
}
