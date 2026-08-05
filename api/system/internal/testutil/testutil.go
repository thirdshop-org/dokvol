package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/pressly/goose/v3"

	_ "github.com/mattn/go-sqlite3"
)

const (
	DockerDataRoot = "/var/lib/docker"
	AlpineImage    = "alpine"
	PollInterval   = 2 * time.Second
)

func DockerClient(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.New(client.FromEnv)
	if err != nil {
		t.Skipf("docker client: %v (skip)", err)
	}
	if _, err := cli.Ping(context.Background(), client.PingOptions{}); err != nil {
		t.Skipf("docker ping: %v (skip)", err)
	}
	return cli
}

func PullImage(t *testing.T, ref string) {
	t.Helper()
	RunOrFatal(t, "docker", "pull", ref)
}

func CreateVolume(t *testing.T, cli *client.Client, name string) {
	t.Helper()
	_, err := cli.VolumeCreate(context.Background(), client.VolumeCreateOptions{Name: name})
	if err != nil {
		t.Fatalf("create volume %s: %v", name, err)
	}
	t.Cleanup(func() {
		_, _ = cli.VolumeRemove(context.Background(), name, client.VolumeRemoveOptions{Force: true})
	})
}

func CreateContainer(t *testing.T, cli *client.Client, name, image string, mounts []mount.Mount, cmd []string) {
	t.Helper()
	result, err := cli.ContainerCreate(context.Background(), client.ContainerCreateOptions{
		Name: name,
		Config: &container.Config{
			Image: image,
			Cmd:   cmd,
		},
		HostConfig: &container.HostConfig{
			Mounts: mounts,
		},
	})
	if err != nil {
		t.Fatalf("create container %s: %v", name, err)
	}
	cleanupName := name
	t.Cleanup(func() {
		RunOrFatal(t, "docker", "stop", "--time", "5", cleanupName)
		_, _ = cli.ContainerRemove(context.Background(), result.ID, client.ContainerRemoveOptions{Force: true})
	})
}

func StartContainer(t *testing.T, cli *client.Client, name string) {
	t.Helper()
	if _, err := cli.ContainerStart(context.Background(), name, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("start container %s: %v", name, err)
	}
}

func WaitContainerRunning(t *testing.T, cli *client.Client, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		r, err := cli.ContainerInspect(context.Background(), name, client.ContainerInspectOptions{})
		if err == nil && r.Container.State.Status == "running" {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("container %s not running after 30s", name)
}

func ExecCmd(t *testing.T, cli *client.Client, ctr, workdir string, cmd []string) {
	t.Helper()
	r, err := cli.ExecCreate(context.Background(), ctr, client.ExecCreateOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   workdir,
	})
	if err != nil {
		t.Fatalf("exec create in %s: %v", ctr, err)
	}
	if _, err := cli.ExecStart(context.Background(), r.ID, client.ExecStartOptions{}); err != nil {
		t.Fatalf("exec start in %s: %v", ctr, err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		info, err := cli.ExecInspect(context.Background(), r.ID, client.ExecInspectOptions{})
		if err != nil {
			t.Fatalf("exec inspect: %v", err)
		}
		if !info.Running {
			if info.ExitCode != 0 {
				t.Fatalf("exec %v exited code %d", cmd, info.ExitCode)
			}
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("exec %v timed out", cmd)
}

func WriteData(t *testing.T, cli *client.Client, ctr, path string, mb int) {
	t.Helper()
	ExecCmd(t, cli, ctr, "", []string{
		"dd", "if=/dev/zero", fmt.Sprintf("of=%s", path),
		"bs=1M", fmt.Sprintf("count=%d", mb),
	})
}

// loopMountRetries/loopMountRetryDelay absorb transient "failed to set up
// loop device" errors: loop devices are a single, un-namespaced host-kernel
// resource pool (8 by default), shared by every container on the host — a
// sibling test (or, on shared CI infra, an entirely unrelated job) can hold
// one for a few seconds right as this one asks for a free slot.
const loopMountRetries = 5
const loopMountRetryDelay = 2 * time.Second

func CreateLoopDrive(t *testing.T, sizeMB int) string {
	t.Helper()
	img := filepath.Join(t.TempDir(), "disk.img")
	mnt := filepath.Join(t.TempDir(), "mnt")
	RunOrFatal(t, "dd", "if=/dev/zero", fmt.Sprintf("of=%s", img), "bs=1M", fmt.Sprintf("count=%d", sizeMB))
	RunOrFatal(t, "mkfs.ext4", "-F", img)
	MustMkdir(t, mnt)
	_ = exec.Command("modprobe", "loop").Run()

	var lastErr error
	for attempt := 1; attempt <= loopMountRetries; attempt++ {
		out, err := exec.Command("mount", "-o", "loop", img, mnt).CombinedOutput()
		if err == nil {
			t.Cleanup(func() { RunOrFatal(t, "umount", mnt) })
			return mnt
		}
		lastErr = fmt.Errorf("mount -o loop %s %s: %w\n%s", img, mnt, err, out)
		time.Sleep(loopMountRetryDelay)
	}
	t.Fatalf("%v (gave up after %d attempts)", lastErr, loopMountRetries)
	return ""
}

func RunOrFatal(t *testing.T, name string, args ...string) {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func MustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func InitTestDB(t *testing.T) *db.Queries {
	t.Helper()
	conn, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	queries := db.New(conn)
	goose.SetBaseFS(nil)
	goose.SetDialect("sqlite3")

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "..", "migrations")
	if err := goose.Up(conn, migrationsDir); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return queries
}

func WaitForDrive(t *testing.T, mountpoint string) *system.DriveInfo {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		drives := system.GetDrives()
		t.Logf("GetDrives found %d drives", len(drives))
		for _, d := range drives {
			t.Logf("  drive: device=%s mount=%s fstype=%s total_gb=%d", d.Device, d.Mountpoint, d.Fstype, d.TotalGB)
			if d.Mountpoint == mountpoint {
				return &d
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	out, _ := exec.Command("mount").CombinedOutput()
	t.Logf("--- mount output ---\n%s", out)
	info, _ := os.ReadFile("/proc/self/mountinfo")
	t.Logf("--- mountinfo ---\n%s", info)
	t.Fatalf("drive %s never appeared in GetDrives()", mountpoint)
	return nil
}

func WaitForApp(t *testing.T, appName string) *system.Application {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s, err := system.New()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		for i := range s.Applications {
			if s.Applications[i].Name == appName {
				return &s.Applications[i]
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("app %s never discovered", appName)
	return nil
}

func BuildVolumeOpts(app *system.Application, drive *system.DriveInfo) []system.ApplicationVolumeOptions {
	opts := make([]system.ApplicationVolumeOptions, len(app.DockerVolumes))
	for i, v := range app.DockerVolumes {
		opts[i] = system.ApplicationVolumeOptions{
			VolumeDetail:     v,
			DestinationDrive: *drive,
		}
	}
	return opts
}

func PollJob(t *testing.T, mm *system.MigrationManager, jobID string, timeout time.Duration) *system.Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := mm.GetJob(context.Background(), jobID)
		if err != nil {
			time.Sleep(PollInterval)
			continue
		}
		switch job.Status {
		case system.JobCompleted, system.JobFailed:
			return job
		}
		time.Sleep(PollInterval)
	}
	t.Fatalf("job %s not done after %v", jobID, timeout)
	return nil
}

func SymlinkTarget(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

func VolumeDataRoot(volName string) string {
	return filepath.Join(DockerDataRoot, "volumes", volName, "_data")
}

func FindVolume(volumes []system.VolumeRow, name string) *system.VolumeRow {
	for i := range volumes {
		if volumes[i].VolumeName == name {
			return &volumes[i]
		}
	}
	return nil
}
