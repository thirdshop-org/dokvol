package system

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moby/moby/client"
	"github.com/shirou/gopsutil/v4/disk"
)

type VolumeDetail struct {
	ContainerName string
	Name          string // Nom du volume Docker
	Type          string // bind ou volume
	Source        string // Chemin sur le serveur (ex: /var/lib/docker/volumes/...)
	Destination   string // Chemin dans le conteneur (ex: /var/www/html)
	SystemDrive   *DriveInfo
}

func (s *System) GetDockerVolumes() []VolumeDetail {
	fmt.Println("🔎 Scan des volumes Docker en cours...")

	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	var results []VolumeDetail

	for _, ctr := range containers.Items {
		// On prend le premier nom du conteneur (Docker met souvent un "/" devant)
		cName := "Unknown"
		if len(ctr.Names) > 0 { // Pour des raisons histo de docker un container pouvait avoir plusieurs noms actuellement dans les dernières version seulement le premier contient un nom
			cName = ctr.Names[0]
		}

		for _, mount := range ctr.Mounts {
			// C'est ici que la magie de DokVol opère :
			// mount.Source est le chemin REEL sur ton disque dur (p1, p2, etc.)

			detail := VolumeDetail{
				ContainerName: cName,
				Name:          mount.Name,
				Type:          string(mount.Type),
				Source:        mount.Source,
				Destination:   mount.Destination,
			}

			results = append(results, detail)

		}
	}
	return results
}

type BasicContainerInfos struct {
	ID   string
	Name string
}

func (s *System) GetDockerContainers() []BasicContainerInfos {
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	basicContainers := make([]BasicContainerInfos, len(containers.Items))

	for index, ctr := range containers.Items {
		// On prend le premier nom du conteneur (Docker met souvent un "/" devant)
		cName := "Unknown"
		if len(ctr.Names) > 0 { // Pour des raisons histo de docker un container pouvait avoir plusieurs noms actuellement dans les dernières version seulement le premier contient un nom
			cName = ctr.Names[0]
		}

		basicContainers[index].ID = ctr.ID
		basicContainers[index].Name = cName

	}

	return basicContainers

}

type ApplicationVolumes struct {
	ContainerName string
	Volumes       []VolumeDetail
}

func GetDockerVolumesByContainers() []ApplicationVolumes {

	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	applications := make([]ApplicationVolumes, 0, len(containers.Items))

	for _, ctr := range containers.Items {

		applicationName := "Unknown"
		if len(ctr.Names) > 0 { // Pour des raisons histo de docker un container pouvait avoir plusieurs noms actuellement dans les dernières version seulement le premier contient un nom
			applicationName = ctr.Names[0]
		}

		volumesDetails := make([]VolumeDetail, 0, len(ctr.Mounts))

		for _, mount := range ctr.Mounts {
			// C'est ici que la magie de DokVol opère :
			// mount.Source est le chemin REEL sur ton disque dur (p1, p2, etc.)

			detail := VolumeDetail{
				ContainerName: applicationName,
				Name:          mount.Name,
				Type:          string(mount.Type),
				Source:        mount.Source,
				Destination:   mount.Destination,
			}

			volumesDetails = append(volumesDetails, detail)

		}

		applications = append(applications, ApplicationVolumes{
			ContainerName: applicationName,
			Volumes:       volumesDetails,
		})

	}

	return applications

}

func (v *VolumeDetail) GetVolumeSize() error {
	usage, err := disk.Usage(v.Source)
	if err != nil {
		return err
	}
	fmt.Println(usage)
	return nil
}

func GetApplicationsDetails(drives []DriveInfo) []Application {

	sort.Slice(drives, func(i, j int) bool {
		return len(drives[i].Mountpoint) > len(drives[j].Mountpoint)
	})

	containers := GetDockerVolumesByContainers()

	applications := make([]Application, len(containers))

	for index, container := range containers {

		volumes := make([]VolumeDetail, len(container.Volumes))

		for i, volume := range container.Volumes {

			var bestMatchDrive *DriveInfo

			for _, drive := range drives {
				// On prépare le point de montage pour éviter les faux positifs
				m := drive.Mountpoint
				if m != "/" && !strings.HasSuffix(m, "/") {
					m += "/"
				}

				// On vérifie si c'est le même chemin ou un sous-dossier
				if volume.Source == drive.Mountpoint || strings.HasPrefix(volume.Source, m) {
					bestMatchDrive = &drive
					break
				}
			}

			volume.SystemDrive = bestMatchDrive

			volumes[i] = volume

		}

		applications[index].DockerVolumes = volumes
		applications[index].Name = container.ContainerName

	}

	return applications

}
