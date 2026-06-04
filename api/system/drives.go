package system

import (
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// DriveInfo représentera une partition prête pour l'interface Vue 3
type DriveInfo struct {
	Device     string  `json:"device"`
	Mountpoint string  `json:"mountpoint"`
	Fstype     string  `json:"fstype"`
	TotalGB    uint64  `json:"total_gb"`
	FreeGB     uint64  `json:"free_gb"`
	UsedPct    float64 `json:"used_pct"`
}

func GetDrives() []DriveInfo {
	var driveList []DriveInfo

	// 1. On récupère toutes les partitions (true = y compris les bind mounts dans les containers)
	partitions, err := disk.Partitions(true)
	if err != nil {
		fmt.Printf("Erreur partitions: %v\n", err)
		return nil
	}

	seenDevices := make(map[string]bool)

	for _, p := range partitions {
		// 2. FILTRAGE : On ignore ce qui n'est pas intéressant pour Docker
		isBoot := strings.HasPrefix(p.Mountpoint, "/boot")
		isEFI := strings.HasPrefix(p.Mountpoint, "/efi")
		isEtc := strings.HasPrefix(p.Mountpoint, "/etc")

		// On ne garde que les systèmes de fichiers "réels"
		isValidFS := p.Fstype == "ext4" || p.Fstype == "xfs" || p.Fstype == "btrfs"

		if !isBoot && !isEFI && !isEtc && isValidFS && !seenDevices[p.Device] {
			seenDevices[p.Device] = true

			usage, err := disk.Usage(p.Mountpoint)
			if err != nil {
				continue
			}

			info := DriveInfo{
				Device:     p.Device,
				Mountpoint: p.Mountpoint,
				Fstype:     p.Fstype,
				TotalGB:    usage.Total / 1024 / 1024 / 1024,
				FreeGB:     usage.Free / 1024 / 1024 / 1024,
				UsedPct:    usage.UsedPercent,
			}

			driveList = append(driveList, info)
		}
	}

	return driveList
}
