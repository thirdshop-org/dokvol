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

	// 1. On récupère toutes les partitions
	partitions, err := disk.Partitions(false)
	if err != nil {
		fmt.Printf("Erreur partitions: %v\n", err)
		return nil
	}

	for _, p := range partitions {
		// 2. FILTRAGE : On ignore ce qui n'est pas intéressant pour Docker
		isBoot := strings.HasPrefix(p.Mountpoint, "/boot")
		isEFI := strings.HasPrefix(p.Mountpoint, "/efi")

		// On ne garde que les systèmes de fichiers "réels"
		// On peut ajouter "btrfs", "xfs", "zfs" selon ton besoin
		isValidFS := p.Fstype == "ext4" || p.Fstype == "xfs" || p.Fstype == "btrfs"

		if !isBoot && !isEFI && isValidFS {
			// 3. On récupère l'usage spécifique à CE point de montage
			usage, err := disk.Usage(p.Mountpoint)
			if err != nil {
				continue
			}

			// 4. On remplit notre structure
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
