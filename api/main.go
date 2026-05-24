package main

import (
	"dokvol/api/system"
	"fmt"
)

func main() {

	system := system.New()

	applications := system.GetApplicationsDetails()

	for _, application := range applications {
		groups := application.GroupVolumesBySystemDrives()
		for _, group := range groups {
			fmt.Println(group)
		}
	}

	// fmt.Println(containers)

	// fmt.Println("--- Disques utilisables pour DokVol ---")

	// volumesDrives := MatchVolumesAndDrives(drives)

	// fmt.Println(volumesDrives)

}
