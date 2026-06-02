package system

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	"dokvol/api/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	dockerDataRoot = "/var/lib/docker"
	alpineImage    = "alpine"
	pollInterval   = 2 * time.Second
)

func dockerClient(t *testing.T) *client.Client {
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

func pullImage(t *testing.T, ref string) {
	t.Helper()
	runOrFatal(t, "docker", "pull", ref)
}

func createVolume(t *testing.T, cli *client.Client, name string) {
	t.Helper()
	_, err := cli.VolumeCreate(context.Background(), client.VolumeCreateOptions{Name: name})
	if err != nil {
		t.Fatalf("create volume %s: %v", name, err)
	}
	t.Cleanup(func() {
		_, _ = cli.VolumeRemove(context.Background(), name, client.VolumeRemoveOptions{Force: true})
	})
}

func createContainer(t *testing.T, cli *client.Client, name, image string, mounts []mount.Mount, cmd []string) {
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
		runOrFatal(t, "docker", "stop", "--time", "5", cleanupName)
		_, _ = cli.ContainerRemove(context.Background(), result.ID, client.ContainerRemoveOptions{Force: true})
	})
}

func startContainer(t *testing.T, cli *client.Client, name string) {
	t.Helper()
	if _, err := cli.ContainerStart(context.Background(), name, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("start container %s: %v", name, err)
	}
}

func waitContainerRunning(t *testing.T, cli *client.Client, name string) {
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

func execCmd(t *testing.T, cli *client.Client, ctr, workdir string, cmd []string) {
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

func writeData(t *testing.T, cli *client.Client, ctr, path string, mb int) {
	t.Helper()
	execCmd(t, cli, ctr, "", []string{
		"dd", "if=/dev/zero", fmt.Sprintf("of=%s", path),
		"bs=1M", fmt.Sprintf("count=%d", mb),
	})
}

// createLoopDrive creates an ext4 loop-backed mount that GetDrives() detects naturally.
func createLoopDrive(t *testing.T, sizeMB int) string {
	t.Helper()
	img := filepath.Join(t.TempDir(), "disk.img")
	mnt := filepath.Join(t.TempDir(), "mnt")
	runOrFatal(t, "dd", "if=/dev/zero", fmt.Sprintf("of=%s", img), "bs=1M", fmt.Sprintf("count=%d", sizeMB))
	runOrFatal(t, "mkfs.ext4", "-F", img)
	mustMkdir(t, mnt)
	_ = exec.Command("modprobe", "loop").Run()
	runOrFatal(t, "mount", "-o", "loop", img, mnt)
	t.Cleanup(func() { runOrFatal(t, "umount", mnt) })
	return mnt
}

func runOrFatal(t *testing.T, name string, args ...string) {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func initTestDB(t *testing.T) *db.Queries {
	t.Helper()
	conn, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	queries := db.New(conn)
	goose.SetBaseFS(nil)
	goose.SetDialect("sqlite3")
	if err := goose.Up(conn, "../migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return queries
}

// waitForDrive polls GetDrives until a drive with the given mountpoint appears.
func waitForDrive(t *testing.T, mountpoint string) *DriveInfo {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		drives := GetDrives()
		t.Logf("GetDrives found %d drives", len(drives))
		for _, d := range drives {
			t.Logf("  drive: device=%s mount=%s fstype=%s total_gb=%d", d.Device, d.Mountpoint, d.Fstype, d.TotalGB)
			if d.Mountpoint == mountpoint {
				return &d
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Debug: show actual mounts
	out, _ := exec.Command("mount").CombinedOutput()
	t.Logf("--- mount output ---\n%s", out)
	info, _ := os.ReadFile("/proc/self/mountinfo")
	t.Logf("--- mountinfo ---\n%s", info)
	t.Fatalf("drive %s never appeared in GetDrives()", mountpoint)
	return nil
}

// waitForApp polls New() until an application with the given name is found.
func waitForApp(t *testing.T, appName string) *Application {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s, err := New()
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

// buildVolumeOpts creates one ApplicationVolumeOptions per volume, all targeting the given drive.
func buildVolumeOpts(app *Application, drive *DriveInfo) []ApplicationVolumeOptions {
	opts := make([]ApplicationVolumeOptions, len(app.DockerVolumes))
	for i, v := range app.DockerVolumes {
		opts[i] = ApplicationVolumeOptions{
			VolumeDetail:     v,
			DestinationDrive: *drive,
		}
	}
	return opts
}

// pollJob polls GetJob until its status is completed or failed.
func pollJob(t *testing.T, mm *MigrationManager, jobID string, timeout time.Duration) *Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := mm.GetJob(context.Background(), jobID)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		switch job.Status {
		case JobCompleted, JobFailed:
			return job
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("job %s not done after %v", jobID, timeout)
	return nil
}

// symlinkTarget returns the target of a symlink, or empty string if not a symlink.
func symlinkTarget(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

// volumeDataRoot returns the on-disk path of a Docker named volume.
func volumeDataRoot(volName string) string {
	return filepath.Join(dockerDataRoot, "volumes", volName, "_data")
}

// ---------------------------------------------------------------------------
// 1. Single volume migration (nominal + empty)
// ---------------------------------------------------------------------------

func TestE2E_SingleVolume(t *testing.T) {
	ctx := context.Background()
	cli := dockerClient(t)
	pullImage(t, alpineImage)

	suffix := uuid.New().String()[:8]
	volName := "e2e-single-vol-" + suffix
	ctrName := "/e2e-single-ctr-" + suffix

	createVolume(t, cli, volName)
	createContainer(t, cli, ctrName, alpineImage,
		[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
		[]string{"sleep", "9999"},
	)
	startContainer(t, cli, ctrName)
	waitContainerRunning(t, cli, ctrName)
	writeData(t, cli, ctrName, "/data/test.bin", 10)

	driveMount := createLoopDrive(t, 100)
	drive := waitForDrive(t, driveMount)
	app := waitForApp(t, ctrName)

	db := initTestDB(t)
	mm := NewMigrationManager(db)
	volOpts := buildVolumeOpts(app, drive)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("job %s started", jobID)

	job := pollJob(t, mm, jobID, 90*time.Second)
	if job.Status != JobCompleted {
		t.Fatalf("expected completed, got %s (volumes: %+v)", job.Status, job.Volumes)
	}
	if len(job.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(job.Volumes))
	}
	if job.Volumes[0].Step != StepCompleted {
		t.Fatalf("expected StepCompleted, got %s", job.Volumes[0].Step)
	}
	if job.Volumes[0].Transferred == 0 {
		t.Fatal("Transferred == 0, expected > 0")
	}

	// Source should now be a symlink pointing to dest
	vol := app.DockerVolumes[0]
	source := vol.Source
	target := symlinkTarget(t, source)
	if target == "" {
		t.Fatalf("source %s is not a symlink", source)
	}
	if !strings.Contains(target, driveMount) {
		t.Fatalf("symlink %s -> %s does not point to drive %s", source, target, driveMount)
	}

	// Container should be running
	info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if info.Container.State.Status != "running" {
		t.Fatalf("container status %s, expected running", info.Container.State.Status)
	}
}

// ---------------------------------------------------------------------------
// 2. Full application migration
// ---------------------------------------------------------------------------

func TestE2E_FullApplication(t *testing.T) {
	ctx := context.Background()
	cli := dockerClient(t)
	pullImage(t, alpineImage)

	suffix := uuid.New().String()[:8]
	ctrName := "/e2e-full-ctr-" + suffix
	volNames := []string{
		"e2e-full-vol-a-" + suffix,
		"e2e-full-vol-b-" + suffix,
		"e2e-full-vol-c-" + suffix,
	}

	for _, v := range volNames {
		createVolume(t, cli, v)
	}
	createContainer(t, cli, ctrName, alpineImage,
		[]mount.Mount{
			{Type: mount.TypeVolume, Source: volNames[0], Target: "/data/a"},
			{Type: mount.TypeVolume, Source: volNames[1], Target: "/data/b"},
			{Type: mount.TypeVolume, Source: volNames[2], Target: "/data/c"},
		},
		[]string{"sleep", "9999"},
	)
	startContainer(t, cli, ctrName)
	waitContainerRunning(t, cli, ctrName)
	writeData(t, cli, ctrName, "/data/a/data.bin", 5)
	writeData(t, cli, ctrName, "/data/b/data.bin", 10)
	writeData(t, cli, ctrName, "/data/c/data.bin", 15)

	driveMount := createLoopDrive(t, 200)
	drive := waitForDrive(t, driveMount)
	app := waitForApp(t, ctrName)

	if len(app.DockerVolumes) != 3 {
		t.Fatalf("expected 3 docker volumes, got %d", len(app.DockerVolumes))
	}

	db := initTestDB(t)
	mm := NewMigrationManager(db)
	volOpts := buildVolumeOpts(app, drive)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("job %s started", jobID)

	job := pollJob(t, mm, jobID, 120*time.Second)
	if job.Status != JobCompleted {
		t.Fatalf("expected completed, got %s", job.Status)
	}
	if len(job.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(job.Volumes))
	}

	for i := range job.Volumes {
		if job.Volumes[i].Step != StepCompleted {
			t.Fatalf("volume %s step %s, expected completed", job.Volumes[i].VolumeName, job.Volumes[i].Step)
		}
	}

	// All sources should be symlinks
	for _, v := range app.DockerVolumes {
		target := symlinkTarget(t, v.Source)
		if target == "" {
			t.Fatalf("volume %s source %s is not a symlink", v.Name, v.Source)
		}
		if !strings.HasPrefix(target, driveMount) {
			t.Fatalf("symlink %s -> %s not on drive %s", v.Source, target, driveMount)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Multi-drive: each volume to a different physical drive
// ---------------------------------------------------------------------------

func TestE2E_MultiDrive(t *testing.T) {
	ctx := context.Background()
	cli := dockerClient(t)
	pullImage(t, alpineImage)

	suffix := uuid.New().String()[:8]
	ctrName := "/e2e-multi-ctr-" + suffix
	volNames := []string{
		"e2e-multi-vol-a-" + suffix,
		"e2e-multi-vol-b-" + suffix,
	}

	for _, v := range volNames {
		createVolume(t, cli, v)
	}
	createContainer(t, cli, ctrName, alpineImage,
		[]mount.Mount{
			{Type: mount.TypeVolume, Source: volNames[0], Target: "/data/a"},
			{Type: mount.TypeVolume, Source: volNames[1], Target: "/data/b"},
		},
		[]string{"sleep", "9999"},
	)
	startContainer(t, cli, ctrName)
	waitContainerRunning(t, cli, ctrName)
	writeData(t, cli, ctrName, "/data/a/data.bin", 10)
	writeData(t, cli, ctrName, "/data/b/data.bin", 20)

	driveMount1 := createLoopDrive(t, 100)
	drive1 := waitForDrive(t, driveMount1)
	driveMount2 := createLoopDrive(t, 200)
	drive2 := waitForDrive(t, driveMount2)

	app := waitForApp(t, ctrName)
	if len(app.DockerVolumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(app.DockerVolumes))
	}

	volOpts := []ApplicationVolumeOptions{
		{VolumeDetail: app.DockerVolumes[0], DestinationDrive: *drive1},
		{VolumeDetail: app.DockerVolumes[1], DestinationDrive: *drive2},
	}

	db := initTestDB(t)
	mm := NewMigrationManager(db)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("job %s started", jobID)

	job := pollJob(t, mm, jobID, 120*time.Second)
	if job.Status != JobCompleted {
		t.Fatalf("expected completed, got %s (volumes: %+v)", job.Status, job.Volumes)
	}
	if len(job.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(job.Volumes))
	}
	for i := range job.Volumes {
		if job.Volumes[i].Step != StepCompleted {
			t.Fatalf("volume %s step %s, expected completed", job.Volumes[i].VolumeName, job.Volumes[i].Step)
		}
		if job.Volumes[i].Transferred == 0 {
			t.Fatalf("volume %s Transferred == 0", job.Volumes[i].VolumeName)
		}
	}
	if job.Volumes[0].DestDrive != driveMount1 {
		t.Fatalf("vol0 dest drive %s, expected %s", job.Volumes[0].DestDrive, driveMount1)
	}
	if job.Volumes[1].DestDrive != driveMount2 {
		t.Fatalf("vol1 dest drive %s, expected %s", job.Volumes[1].DestDrive, driveMount2)
	}

	// Volume 0 → drive1
	source0 := app.DockerVolumes[0].Source
	target0 := symlinkTarget(t, source0)
	if target0 == "" {
		t.Fatalf("vol0 source %s is not a symlink", source0)
	}
	if !strings.HasPrefix(target0, driveMount1) {
		t.Fatalf("vol0 symlink %s -> %s does not point to drive %s", source0, target0, driveMount1)
	}
	t.Logf("vol0 symlink: %s -> %s", source0, target0)

	// Volume 1 → drive2
	source1 := app.DockerVolumes[1].Source
	target1 := symlinkTarget(t, source1)
	if target1 == "" {
		t.Fatalf("vol1 source %s is not a symlink", source1)
	}
	if !strings.HasPrefix(target1, driveMount2) {
		t.Fatalf("vol1 symlink %s -> %s does not point to drive %s", source1, target1, driveMount2)
	}
	t.Logf("vol1 symlink: %s -> %s", source1, target1)

	// Container should still be running
	info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if info.Container.State.Status != "running" {
		t.Fatalf("container status %s, expected running", info.Container.State.Status)
	}
}

// ---------------------------------------------------------------------------
// 4. Error handling: OOM / same-drive / rollback
// ---------------------------------------------------------------------------

func TestE2E_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	cli := dockerClient(t)
	pullImage(t, alpineImage)

	t.Run("disk_space", func(t *testing.T) {
		suffix := uuid.New().String()[:8]
		volName := "e2e-oom-vol-" + suffix
		ctrName := "/e2e-oom-ctr-" + suffix

		createVolume(t, cli, volName)
		createContainer(t, cli, ctrName, alpineImage,
			[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
			[]string{"sleep", "9999"},
		)
		startContainer(t, cli, ctrName)
		waitContainerRunning(t, cli, ctrName)
		writeData(t, cli, ctrName, "/data/large.bin", 500)

		driveMount := createLoopDrive(t, 80) // only 80MB, can't hold 500MB
		drive := waitForDrive(t, driveMount)
		app := waitForApp(t, ctrName)

		db := initTestDB(t)
		mm := NewMigrationManager(db)
		volOpts := buildVolumeOpts(app, drive)

		jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job := pollJob(t, mm, jobID, 30*time.Second)
		if job.Status != JobFailed {
			t.Fatalf("expected job failed, got %s", job.Status)
		}
		if len(job.Volumes) == 0 || !strings.Contains(job.Volumes[0].Error, "not enough space") {
			t.Fatalf("expected disk space error, got: %+v", job.Volumes)
		}
	})

	t.Run("insufficient_space_on_one_drive", func(t *testing.T) {
		suffix := uuid.New().String()[:8]
		volNameA := "e2e-spc-vol-a-" + suffix
		volNameB := "e2e-spc-vol-b-" + suffix
		ctrName := "/e2e-spc-ctr-" + suffix

		createVolume(t, cli, volNameA)
		createVolume(t, cli, volNameB)
		createContainer(t, cli, ctrName, alpineImage,
			[]mount.Mount{
				{Type: mount.TypeVolume, Source: volNameA, Target: "/data/a"},
				{Type: mount.TypeVolume, Source: volNameB, Target: "/data/b"},
			},
			[]string{"sleep", "9999"},
		)
		startContainer(t, cli, ctrName)
		waitContainerRunning(t, cli, ctrName)
		writeData(t, cli, ctrName, "/data/a/data.bin", 10)
		writeData(t, cli, ctrName, "/data/b/large.bin", 150)

		driveMount1 := createLoopDrive(t, 500) // plenty for 10MB
		drive1 := waitForDrive(t, driveMount1)
		driveMount2 := createLoopDrive(t, 80) // not enough for 150MB
		drive2 := waitForDrive(t, driveMount2)

		app := waitForApp(t, ctrName)
		if len(app.DockerVolumes) != 2 {
			t.Fatalf("expected 2 volumes, got %d", len(app.DockerVolumes))
		}

		db := initTestDB(t)
		mm := NewMigrationManager(db)
		volOpts := []ApplicationVolumeOptions{
			{VolumeDetail: app.DockerVolumes[0], DestinationDrive: *drive1},
			{VolumeDetail: app.DockerVolumes[1], DestinationDrive: *drive2},
		}

		jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job := pollJob(t, mm, jobID, 30*time.Second)
		if job.Status != JobFailed {
			t.Fatalf("expected job failed, got %s", job.Status)
		}
		if len(job.Volumes) == 0 || !strings.Contains(job.Volumes[0].Error, "not enough space") {
			t.Fatalf("expected disk space error, got: %+v", job.Volumes)
		}

		// Container must still be running
		info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
		if err != nil {
			t.Fatalf("inspect: %v", err)
		}
		if info.Container.State.Status != "running" {
			t.Fatalf("container not running, status: %s", info.Container.State.Status)
		}

		// No symlinks — nothing was migrated
		for _, v := range app.DockerVolumes {
			if target := symlinkTarget(t, v.Source); target != "" {
				t.Fatalf("source is a symlink after failed migration: %s -> %s", v.Source, target)
			}
		}
	})

	t.Run("same_drive", func(t *testing.T) {
		suffix := uuid.New().String()[:8]
		volName := "e2e-same-vol-" + suffix
		ctrName := "/e2e-same-ctr-" + suffix

		createVolume(t, cli, volName)
		createContainer(t, cli, ctrName, alpineImage,
			[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
			[]string{"sleep", "9999"},
		)
		startContainer(t, cli, ctrName)
		waitContainerRunning(t, cli, ctrName)

		// Create a loop drive
		driveMount := createLoopDrive(t, 100)
		drive := waitForDrive(t, driveMount)
		app := waitForApp(t, ctrName)

		db := initTestDB(t)
		mm := NewMigrationManager(db)
		volOpts := buildVolumeOpts(app, drive)

		// First migration: migrate to the loop drive → should succeed
		jobID1, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job1 := pollJob(t, mm, jobID1, 60*time.Second)
		if job1.Status != JobCompleted {
			t.Fatalf("expected first migration to succeed, got %s", job1.Status)
		}

		// Wait for the app to be rediscovered (volume source is now a symlink)
		app2 := waitForApp(t, ctrName)
		if len(app2.DockerVolumes) == 0 {
			t.Fatal("app has no volumes")
		}

		// Second migration: migrate to the SAME drive → should fail
		volOpts2 := buildVolumeOpts(app2, drive)
		jobID2, err := mm.StartJob(ctx, app2.Name, *app2, volOpts2)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job2 := pollJob(t, mm, jobID2, 30*time.Second)
		if job2.Status != JobFailed {
			t.Fatalf("expected second migration to fail, got %s", job2.Status)
		}
		if len(job2.Volumes) == 0 || !strings.Contains(job2.Volumes[0].Error, "already on drive") {
			t.Fatalf("expected same-drive error, got: %+v", job2.Volumes)
		}
	})

	t.Run("validation_failure_keeps_container", func(t *testing.T) {
		suffix := uuid.New().String()[:8]
		volName := "e2e-noroll-vol-" + suffix
		ctrName := "/e2e-noroll-ctr-" + suffix

		createVolume(t, cli, volName)
		createContainer(t, cli, ctrName, alpineImage,
			[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
			[]string{"sleep", "9999"},
		)
		startContainer(t, cli, ctrName)
		waitContainerRunning(t, cli, ctrName)
		writeData(t, cli, ctrName, "/data/important.bin", 15)

		driveMount := createLoopDrive(t, 8) // too small for 15MB
		drive := waitForDrive(t, driveMount)
		app := waitForApp(t, ctrName)

		db := initTestDB(t)
		mm := NewMigrationManager(db)
		volOpts := buildVolumeOpts(app, drive)

		jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job := pollJob(t, mm, jobID, 30*time.Second)
		if job.Status != JobFailed {
			t.Fatalf("expected job failed, got %s", job.Status)
		}

		// Container must still be running
		info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
		if err != nil {
			t.Fatalf("inspect: %v", err)
		}
		if info.Container.State.Status != "running" {
			t.Fatalf("container not running, status: %s", info.Container.State.Status)
		}

		// Source must NOT be a symlink (data unchanged)
		source := app.DockerVolumes[0].Source
		if target := symlinkTarget(t, source); target != "" {
			t.Fatalf("source is a symlink after failed migration: %s -> %s", source, target)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Partial failure: 2 volumes, first succeeds, second fails mid-migration
// ---------------------------------------------------------------------------

func TestE2E_PartialFailure(t *testing.T) {
	ctx := context.Background()
	cli := dockerClient(t)
	pullImage(t, alpineImage)

	suffix := uuid.New().String()[:8]
	volName1 := "e2e-part-vol1-" + suffix
	volName2 := "e2e-part-vol2-" + suffix
	ctrName := "/e2e-part-ctr-" + suffix

	createVolume(t, cli, volName1)
	createVolume(t, cli, volName2)
	createContainer(t, cli, ctrName, alpineImage,
		[]mount.Mount{
			{Type: mount.TypeVolume, Source: volName1, Target: "/data1"},
			{Type: mount.TypeVolume, Source: volName2, Target: "/data2"},
		},
		[]string{"sleep", "9999"},
	)
	startContainer(t, cli, ctrName)
	waitContainerRunning(t, cli, ctrName)
	writeData(t, cli, ctrName, "/data1/vol1.bin", 5)
	writeData(t, cli, ctrName, "/data2/vol2.bin", 100)

	driveMount := createLoopDrive(t, 300)
	drive := waitForDrive(t, driveMount)
	app := waitForApp(t, ctrName)

	if len(app.DockerVolumes) != 2 {
		t.Fatalf("expected 2 docker volumes, got %d", len(app.DockerVolumes))
	}

	db := initTestDB(t)
	mm := NewMigrationManager(db)
	volOpts := buildVolumeOpts(app, drive)

	// Ensure volume 2's source path is known before migration
	vol2Source := app.DockerVolumes[1].Source
	t.Logf("vol2 source path: %s", vol2Source)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("partial failure job %s started", jobID)

	// Poll frequently to catch vol1 completed BEFORE vol2 migrates
	var mu sync.Mutex
	chmodDone := false

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		job, err := mm.GetJob(ctx, jobID)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		mu.Lock()
		if !chmodDone && len(job.Volumes) > 0 && job.Volumes[0].Step == StepCompleted {
			// Volume 1 is done → rename volume 2 source to make it inaccessible
			t.Log("vol1 completed, breaking vol2 now")
			sourceDir := volumeDataRoot(volName2)
			backupDir := sourceDir + ".bak"
			if err := os.Rename(sourceDir, backupDir); err != nil {
				t.Fatalf("rename %s -> %s: %v", sourceDir, backupDir, err)
			}
			chmodDone = true
		}
		mu.Unlock()

		if job.Status == JobCompleted || job.Status == JobFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	job := pollJob(t, mm, jobID, 10*time.Second) // should be nearly done

	if job.Status != JobFailed {
		t.Fatalf("expected job failed, got %s", job.Status)
	}

	// Volume 1 should be completed
	var vol1, vol2 *VolumeRow
	for i := range job.Volumes {
		if job.Volumes[i].VolumeName == volName1 {
			vol1 = &job.Volumes[i]
		}
		if job.Volumes[i].VolumeName == volName2 {
			vol2 = &job.Volumes[i]
		}
	}
	if vol1 == nil || vol2 == nil {
		t.Fatal("volumes not found in job result")
	}

	if vol1.Step != StepCompleted {
		t.Fatalf("vol1 step %s, expected completed", vol1.Step)
	}
	if vol1.Transferred == 0 {
		t.Fatal("vol1 Transferred == 0")
	}
	if vol2.Step != StepFailed {
		t.Fatalf("vol2 step %s, expected failed", vol2.Step)
	}
	if vol2.Error == "" {
		t.Fatal("vol2 error is empty")
	}
	t.Logf("vol2 error: %s", vol2.Error)

	// Volume 1 source should be a symlink to the drive
	source1 := app.DockerVolumes[0].Source
	target1 := symlinkTarget(t, source1)
	if target1 == "" {
		t.Fatalf("vol1 source is not a symlink")
	}
	if !strings.HasPrefix(target1, driveMount) {
		t.Fatalf("vol1 symlink not on drive: %s", target1)
	}

	// Volume 2 source should NOT be a symlink (migration failed, rollback)
	source2 := app.DockerVolumes[1].Source
	target2 := symlinkTarget(t, source2)
	if target2 != "" {
		t.Fatalf("vol2 source should not be a symlink after failure, got: %s -> %s", source2, target2)
	}

	// Cleanup: restore vol2 source so Docker volume removal works
	sourceDir := volumeDataRoot(volName2)
	backupDir := sourceDir + ".bak"
	if _, err := os.Stat(backupDir); err == nil {
		if err := os.Rename(backupDir, sourceDir); err != nil {
			t.Logf("warning: failed to restore %s: %v", sourceDir, err)
		}
	}
}
