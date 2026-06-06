package system

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"github.com/shirou/gopsutil/v4/disk"
)

type VolumeDetail struct {
	ContainerName       string     `json:"ContainerName"`
	Name                string     `json:"Name"`           // Nom du volume Docker
	Type                string     `json:"Type"`           // bind ou volume
	Source              string     `json:"Source"`         // Chemin sur le serveur (ex: /var/lib/docker/volumes/...)
	Destination         string     `json:"Destination"`    // Chemin dans le conteneur (ex: /var/www/html)
	SystemDrive         *DriveInfo `json:"SystemDrive"`
	IsMigratable        bool       `json:"IsMigratable"`
	MigratedDriveMountpoint string `json:"MigratedDriveMountpoint,omitempty"`
	MigratedDestPath    string     `json:"MigratedDestPath,omitempty"`
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
				IsMigratable:  mount.Source != "" && string(mount.Type) != "tmpfs",
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
	Status        string
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
				IsMigratable:  mount.Source != "" && string(mount.Type) != "tmpfs",
			}

			volumesDetails = append(volumesDetails, detail)

		}

		applications = append(applications, ApplicationVolumes{
			ContainerName: applicationName,
			Status:        string(ctr.State),
			Volumes:       volumesDetails,
		})

	}

	return applications

}

func DeleteVolumes(vols []VolumeDetail) []error {
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return []error{fmt.Errorf("docker client: %w", err)}
	}
	defer apiClient.Close()

	var errs []error
	for _, v := range vols {
		if v.Type == "volume" && v.Name != "" {
			if _, err := apiClient.VolumeRemove(context.Background(), v.Name, client.VolumeRemoveOptions{Force: true}); err != nil {
				errs = append(errs, fmt.Errorf("volume %q: %w", v.Name, err))
			}
		}
	}
	return errs
}

func StopContainer(appName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	killCmd := exec.CommandContext(ctx, "docker", "kill", appName)
	if _, err := killCmd.CombinedOutput(); err != nil {
		// container might already be stopped
	}

	waitCmd := exec.CommandContext(ctx, "docker", "wait", appName)
	out, err := waitCmd.CombinedOutput()
	if err != nil {
		return NewAPIError(
			ErrContainerStopFailed,
			fmt.Sprintf("failed to stop container '%s': %s\n%s", appName, err, out),
			map[string]any{"container": appName},
		)
	}
	return nil
}

func StartContainer(appName string) error {
	cmd := exec.Command("docker", "start", appName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return NewAPIError(
			ErrContainerStartFailed,
			fmt.Sprintf("failed to start container '%s': %s\n%s", appName, err, out),
			map[string]any{"container": appName},
		)
	}
	if err := waitContainerRunning(appName); err != nil {
		return fmt.Errorf("container started but not healthy: %w", err)
	}
	return nil
}

func RestartContainer(appName string) error {
	if err := StopContainer(appName); err != nil {
		return err
	}
	return StartContainer(appName)
}

func waitContainerRunning(appName string) error {
	docker, err := client.New(client.FromEnv)
	if err != nil {
		return NewAPIError(ErrContainerTimeout, fmt.Sprintf("docker client: %s", err), nil)
	}
	defer docker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return NewAPIError(
				ErrContainerTimeout,
				fmt.Sprintf("timeout waiting for container '%s' to be running", appName),
				map[string]any{"container": appName},
			)
		default:
			inspect, err := docker.ContainerInspect(context.Background(), appName, client.ContainerInspectOptions{})
			if err != nil {
				return NewAPIError(
					ErrContainerTimeout,
					fmt.Sprintf("failed to inspect container '%s': %s", appName, err),
					map[string]any{"container": appName},
				)
			}

			switch inspect.Container.State.Status {
			case "running":
				if inspect.Container.State.Health == nil {
					return nil
				}
				if inspect.Container.State.Health.Status == "healthy" {
					return nil
				}
			case "exited", "dead":
				return NewAPIError(
					ErrContainerStopFailed,
					fmt.Sprintf("container '%s' exited unexpectedly", appName),
					map[string]any{"container": appName},
				)
			}

			time.Sleep(2 * time.Second)
		}
	}
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

			// Detect if volume is migrated (source is a symlink to .dokvol/ on a drive)
			if resolved, err := filepath.EvalSymlinks(volume.Source); err == nil {
				if idx := strings.Index(resolved, "/"+DOKVOL_FOLDER+"/"); idx != -1 {
					driveMount := resolved[:idx]
					for _, d := range drives {
						if d.Mountpoint == driveMount {
							volume.MigratedDriveMountpoint = d.Mountpoint
							volume.MigratedDestPath = resolved
							break
						}
					}
					if volume.MigratedDriveMountpoint == "" {
						volume.MigratedDriveMountpoint = driveMount
						volume.MigratedDestPath = resolved
					}
				}
			}

			volumes[i] = volume

		}

		applications[index].DockerVolumes = volumes
		applications[index].Name = container.ContainerName

	}

	return applications

}
