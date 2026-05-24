package main

import (
	"dokvol/api/system"
	"fmt"
)

func main() {

	sys := system.New()

	for _, application := range sys.Applications {

		options := system.MoveStorageOptions{
			DefaultDestinationDrive: &sys.Drives[0],
			Application:             application,
		}

		err := sys.MoveApplicationStorage(options)

		fmt.Println(err)

	}

	// fmt.Println(containers)

	// fmt.Println("--- Disques utilisables pour DokVol ---")

	// volumesDrives := MatchVolumesAndDrives(drives)

	// fmt.Println(volumesDrives)

}
