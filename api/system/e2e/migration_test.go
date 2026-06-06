package e2e

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dokvol/api/system"
	"dokvol/api/system/internal/testutil"

	"github.com/google/uuid"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func TestE2E_SingleVolume(t *testing.T) {
	ctx := context.Background()
	cli := testutil.DockerClient(t)
	testutil.PullImage(t, testutil.AlpineImage)

	suffix := uuid.New().String()[:8]
	volName := "e2e-single-vol-" + suffix
	ctrName := "/e2e-single-ctr-" + suffix

	testutil.CreateVolume(t, cli, volName)
	testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
		[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
		[]string{"sleep", "9999"},
	)
	testutil.StartContainer(t, cli, ctrName)
	testutil.WaitContainerRunning(t, cli, ctrName)
	testutil.WriteData(t, cli, ctrName, "/data/test.bin", 10)

	driveMount := testutil.CreateLoopDrive(t, 100)
	drive := testutil.WaitForDrive(t, driveMount)
	app := testutil.WaitForApp(t, ctrName)

	db := testutil.InitTestDB(t)
	mm := system.NewMigrationManager(db)
	volOpts := testutil.BuildVolumeOpts(app, drive)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("job %s started", jobID)

	job := testutil.PollJob(t, mm, jobID, 90*time.Second)
	if job.Status != system.JobCompleted {
		t.Fatalf("expected completed, got %s (volumes: %+v)", job.Status, job.Volumes)
	}
	if len(job.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(job.Volumes))
	}
	if job.Volumes[0].Step != system.StepCompleted {
		t.Fatalf("expected StepCompleted, got %s", job.Volumes[0].Step)
	}
	if job.Volumes[0].Transferred == 0 {
		t.Fatal("Transferred == 0, expected > 0")
	}

	vol := app.DockerVolumes[0]
	source := vol.Source
	target := testutil.SymlinkTarget(t, source)
	if target == "" {
		t.Fatalf("source %s is not a symlink", source)
	}
	if !strings.Contains(target, driveMount) {
		t.Fatalf("symlink %s -> %s does not point to drive %s", source, target, driveMount)
	}

	info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if info.Container.State.Status != "running" {
		t.Fatalf("container status %s, expected running", info.Container.State.Status)
	}
}

func TestE2E_FullApplication(t *testing.T) {
	ctx := context.Background()
	cli := testutil.DockerClient(t)
	testutil.PullImage(t, testutil.AlpineImage)

	suffix := uuid.New().String()[:8]
	ctrName := "/e2e-full-ctr-" + suffix
	volNames := []string{
		"e2e-full-vol-a-" + suffix,
		"e2e-full-vol-b-" + suffix,
		"e2e-full-vol-c-" + suffix,
	}

	for _, v := range volNames {
		testutil.CreateVolume(t, cli, v)
	}
	testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
		[]mount.Mount{
			{Type: mount.TypeVolume, Source: volNames[0], Target: "/data/a"},
			{Type: mount.TypeVolume, Source: volNames[1], Target: "/data/b"},
			{Type: mount.TypeVolume, Source: volNames[2], Target: "/data/c"},
		},
		[]string{"sleep", "9999"},
	)
	testutil.StartContainer(t, cli, ctrName)
	testutil.WaitContainerRunning(t, cli, ctrName)
	testutil.WriteData(t, cli, ctrName, "/data/a/data.bin", 5)
	testutil.WriteData(t, cli, ctrName, "/data/b/data.bin", 10)
	testutil.WriteData(t, cli, ctrName, "/data/c/data.bin", 15)

	driveMount := testutil.CreateLoopDrive(t, 200)
	drive := testutil.WaitForDrive(t, driveMount)
	app := testutil.WaitForApp(t, ctrName)

	if len(app.DockerVolumes) != 3 {
		t.Fatalf("expected 3 docker volumes, got %d", len(app.DockerVolumes))
	}

	db := testutil.InitTestDB(t)
	mm := system.NewMigrationManager(db)
	volOpts := testutil.BuildVolumeOpts(app, drive)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("job %s started", jobID)

	job := testutil.PollJob(t, mm, jobID, 120*time.Second)
	if job.Status != system.JobCompleted {
		t.Fatalf("expected completed, got %s", job.Status)
	}
	if len(job.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(job.Volumes))
	}

	for i := range job.Volumes {
		if job.Volumes[i].Step != system.StepCompleted {
			t.Fatalf("volume %s step %s, expected completed", job.Volumes[i].VolumeName, job.Volumes[i].Step)
		}
	}

	for _, v := range app.DockerVolumes {
		target := testutil.SymlinkTarget(t, v.Source)
		if target == "" {
			t.Fatalf("volume %s source %s is not a symlink", v.Name, v.Source)
		}
		if !strings.HasPrefix(target, driveMount) {
			t.Fatalf("symlink %s -> %s not on drive %s", v.Source, target, driveMount)
		}
	}
}

func TestE2E_MultiDrive(t *testing.T) {
	ctx := context.Background()
	cli := testutil.DockerClient(t)
	testutil.PullImage(t, testutil.AlpineImage)

	suffix := uuid.New().String()[:8]
	ctrName := "/e2e-multi-ctr-" + suffix
	volNames := []string{
		"e2e-multi-vol-a-" + suffix,
		"e2e-multi-vol-b-" + suffix,
	}

	for _, v := range volNames {
		testutil.CreateVolume(t, cli, v)
	}
	testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
		[]mount.Mount{
			{Type: mount.TypeVolume, Source: volNames[0], Target: "/data/a"},
			{Type: mount.TypeVolume, Source: volNames[1], Target: "/data/b"},
		},
		[]string{"sleep", "9999"},
	)
	testutil.StartContainer(t, cli, ctrName)
	testutil.WaitContainerRunning(t, cli, ctrName)
	testutil.WriteData(t, cli, ctrName, "/data/a/data.bin", 10)
	testutil.WriteData(t, cli, ctrName, "/data/b/data.bin", 20)

	driveMount1 := testutil.CreateLoopDrive(t, 100)
	drive1 := testutil.WaitForDrive(t, driveMount1)
	driveMount2 := testutil.CreateLoopDrive(t, 200)
	drive2 := testutil.WaitForDrive(t, driveMount2)

	app := testutil.WaitForApp(t, ctrName)
	if len(app.DockerVolumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(app.DockerVolumes))
	}

	volOpts := []system.ApplicationVolumeOptions{
		{VolumeDetail: app.DockerVolumes[0], DestinationDrive: *drive1},
		{VolumeDetail: app.DockerVolumes[1], DestinationDrive: *drive2},
	}

	db := testutil.InitTestDB(t)
	mm := system.NewMigrationManager(db)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("job %s started", jobID)

	job := testutil.PollJob(t, mm, jobID, 120*time.Second)
	if job.Status != system.JobCompleted {
		t.Fatalf("expected completed, got %s (volumes: %+v)", job.Status, job.Volumes)
	}
	if len(job.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(job.Volumes))
	}
	for i := range job.Volumes {
		if job.Volumes[i].Step != system.StepCompleted {
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

	source0 := app.DockerVolumes[0].Source
	target0 := testutil.SymlinkTarget(t, source0)
	if target0 == "" {
		t.Fatalf("vol0 source %s is not a symlink", source0)
	}
	if !strings.HasPrefix(target0, driveMount1) {
		t.Fatalf("vol0 symlink %s -> %s does not point to drive %s", source0, target0, driveMount1)
	}
	t.Logf("vol0 symlink: %s -> %s", source0, target0)

	source1 := app.DockerVolumes[1].Source
	target1 := testutil.SymlinkTarget(t, source1)
	if target1 == "" {
		t.Fatalf("vol1 source %s is not a symlink", source1)
	}
	if !strings.HasPrefix(target1, driveMount2) {
		t.Fatalf("vol1 symlink %s -> %s does not point to drive %s", source1, target1, driveMount2)
	}
	t.Logf("vol1 symlink: %s -> %s", source1, target1)

	info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if info.Container.State.Status != "running" {
		t.Fatalf("container status %s, expected running", info.Container.State.Status)
	}
}

func TestE2E_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	cli := testutil.DockerClient(t)
	testutil.PullImage(t, testutil.AlpineImage)

	t.Run("disk_space", func(t *testing.T) {
		suffix := uuid.New().String()[:8]
		volName := "e2e-oom-vol-" + suffix
		ctrName := "/e2e-oom-ctr-" + suffix

		testutil.CreateVolume(t, cli, volName)
		testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
			[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
			[]string{"sleep", "9999"},
		)
		testutil.StartContainer(t, cli, ctrName)
		testutil.WaitContainerRunning(t, cli, ctrName)
		testutil.WriteData(t, cli, ctrName, "/data/large.bin", 500)

		driveMount := testutil.CreateLoopDrive(t, 80)
		drive := testutil.WaitForDrive(t, driveMount)
		app := testutil.WaitForApp(t, ctrName)

		db := testutil.InitTestDB(t)
		mm := system.NewMigrationManager(db)
		volOpts := testutil.BuildVolumeOpts(app, drive)

		jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job := testutil.PollJob(t, mm, jobID, 30*time.Second)
		if job.Status != system.JobFailed {
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

		testutil.CreateVolume(t, cli, volNameA)
		testutil.CreateVolume(t, cli, volNameB)
		testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
			[]mount.Mount{
				{Type: mount.TypeVolume, Source: volNameA, Target: "/data/a"},
				{Type: mount.TypeVolume, Source: volNameB, Target: "/data/b"},
			},
			[]string{"sleep", "9999"},
		)
		testutil.StartContainer(t, cli, ctrName)
		testutil.WaitContainerRunning(t, cli, ctrName)
		testutil.WriteData(t, cli, ctrName, "/data/a/data.bin", 10)
		testutil.WriteData(t, cli, ctrName, "/data/b/large.bin", 150)

		driveMount1 := testutil.CreateLoopDrive(t, 500)
		drive1 := testutil.WaitForDrive(t, driveMount1)
		driveMount2 := testutil.CreateLoopDrive(t, 80)
		drive2 := testutil.WaitForDrive(t, driveMount2)

		app := testutil.WaitForApp(t, ctrName)
		if len(app.DockerVolumes) != 2 {
			t.Fatalf("expected 2 volumes, got %d", len(app.DockerVolumes))
		}

		db := testutil.InitTestDB(t)
		mm := system.NewMigrationManager(db)
		volOpts := []system.ApplicationVolumeOptions{
			{VolumeDetail: app.DockerVolumes[0], DestinationDrive: *drive1},
			{VolumeDetail: app.DockerVolumes[1], DestinationDrive: *drive2},
		}

		jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job := testutil.PollJob(t, mm, jobID, 30*time.Second)
		if job.Status != system.JobFailed {
			t.Fatalf("expected job failed, got %s", job.Status)
		}
		if len(job.Volumes) == 0 || !strings.Contains(job.Volumes[0].Error, "not enough space") {
			t.Fatalf("expected disk space error, got: %+v", job.Volumes)
		}

		info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
		if err != nil {
			t.Fatalf("inspect: %v", err)
		}
		if info.Container.State.Status != "running" {
			t.Fatalf("container not running, status: %s", info.Container.State.Status)
		}

		for _, v := range app.DockerVolumes {
			if target := testutil.SymlinkTarget(t, v.Source); target != "" {
				t.Fatalf("source is a symlink after failed migration: %s -> %s", v.Source, target)
			}
		}
	})

	t.Run("same_drive", func(t *testing.T) {
		suffix := uuid.New().String()[:8]
		volName := "e2e-same-vol-" + suffix
		ctrName := "/e2e-same-ctr-" + suffix

		testutil.CreateVolume(t, cli, volName)
		testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
			[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
			[]string{"sleep", "9999"},
		)
		testutil.StartContainer(t, cli, ctrName)
		testutil.WaitContainerRunning(t, cli, ctrName)

		driveMount := testutil.CreateLoopDrive(t, 100)
		drive := testutil.WaitForDrive(t, driveMount)
		app := testutil.WaitForApp(t, ctrName)

		db := testutil.InitTestDB(t)
		mm := system.NewMigrationManager(db)
		volOpts := testutil.BuildVolumeOpts(app, drive)

		jobID1, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job1 := testutil.PollJob(t, mm, jobID1, 60*time.Second)
		if job1.Status != system.JobCompleted {
			t.Fatalf("expected first migration to succeed, got %s", job1.Status)
		}

		app2 := testutil.WaitForApp(t, ctrName)
		if len(app2.DockerVolumes) == 0 {
			t.Fatal("app has no volumes")
		}

		volOpts2 := testutil.BuildVolumeOpts(app2, drive)
		jobID2, err := mm.StartJob(ctx, app2.Name, *app2, volOpts2)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job2 := testutil.PollJob(t, mm, jobID2, 30*time.Second)
		if job2.Status != system.JobFailed {
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

		testutil.CreateVolume(t, cli, volName)
		testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
			[]mount.Mount{{Type: mount.TypeVolume, Source: volName, Target: "/data"}},
			[]string{"sleep", "9999"},
		)
		testutil.StartContainer(t, cli, ctrName)
		testutil.WaitContainerRunning(t, cli, ctrName)
		testutil.WriteData(t, cli, ctrName, "/data/important.bin", 15)

		driveMount := testutil.CreateLoopDrive(t, 8)
		drive := testutil.WaitForDrive(t, driveMount)
		app := testutil.WaitForApp(t, ctrName)

		db := testutil.InitTestDB(t)
		mm := system.NewMigrationManager(db)
		volOpts := testutil.BuildVolumeOpts(app, drive)

		jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
		if err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		job := testutil.PollJob(t, mm, jobID, 30*time.Second)
		if job.Status != system.JobFailed {
			t.Fatalf("expected job failed, got %s", job.Status)
		}

		info, err := cli.ContainerInspect(ctx, ctrName, client.ContainerInspectOptions{})
		if err != nil {
			t.Fatalf("inspect: %v", err)
		}
		if info.Container.State.Status != "running" {
			t.Fatalf("container not running, status: %s", info.Container.State.Status)
		}

		source := app.DockerVolumes[0].Source
		if target := testutil.SymlinkTarget(t, source); target != "" {
			t.Fatalf("source is a symlink after failed migration: %s -> %s", source, target)
		}
	})
}

func TestE2E_PartialFailure(t *testing.T) {
	ctx := context.Background()
	cli := testutil.DockerClient(t)
	testutil.PullImage(t, testutil.AlpineImage)

	suffix := uuid.New().String()[:8]
	volName1 := "e2e-part-vol1-" + suffix
	volName2 := "e2e-part-vol2-" + suffix
	ctrName := "/e2e-part-ctr-" + suffix

	testutil.CreateVolume(t, cli, volName1)
	testutil.CreateVolume(t, cli, volName2)
	testutil.CreateContainer(t, cli, ctrName, testutil.AlpineImage,
		[]mount.Mount{
			{Type: mount.TypeVolume, Source: volName1, Target: "/data1"},
			{Type: mount.TypeVolume, Source: volName2, Target: "/data2"},
		},
		[]string{"sleep", "9999"},
	)
	testutil.StartContainer(t, cli, ctrName)
	testutil.WaitContainerRunning(t, cli, ctrName)
	testutil.WriteData(t, cli, ctrName, "/data1/vol1.bin", 5)
	testutil.WriteData(t, cli, ctrName, "/data2/vol2.bin", 100)

	driveMount := testutil.CreateLoopDrive(t, 300)
	drive := testutil.WaitForDrive(t, driveMount)
	app := testutil.WaitForApp(t, ctrName)

	if len(app.DockerVolumes) != 2 {
		t.Fatalf("expected 2 docker volumes, got %d", len(app.DockerVolumes))
	}

	db := testutil.InitTestDB(t)
	mm := system.NewMigrationManager(db)
	volOpts := testutil.BuildVolumeOpts(app, drive)

	vol2Source := app.DockerVolumes[1].Source
	t.Logf("vol2 source path: %s", vol2Source)

	jobID, err := mm.StartJob(ctx, app.Name, *app, volOpts)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	t.Logf("partial failure job %s started", jobID)

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
		v1 := testutil.FindVolume(job.Volumes, volName1)
		if !chmodDone && v1 != nil && v1.Step == system.StepCompleted {
			t.Log("vol1 completed, breaking vol2 now")
			sourceDir := testutil.VolumeDataRoot(volName2)
			backupDir := sourceDir + ".bak"
			if err := os.Rename(sourceDir, backupDir); err != nil {
				t.Fatalf("rename %s -> %s: %v", sourceDir, backupDir, err)
			}
			chmodDone = true
		}
		mu.Unlock()

		if job.Status == system.JobCompleted || job.Status == system.JobFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	job := testutil.PollJob(t, mm, jobID, 10*time.Second)

	if job.Status != system.JobFailed {
		t.Fatalf("expected job failed, got %s", job.Status)
	}

	var vol1, vol2 *system.VolumeRow
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

	if vol1.Step != system.StepCompleted {
		t.Fatalf("vol1 step %s, expected completed", vol1.Step)
	}
	if vol1.Transferred == 0 {
		t.Fatal("vol1 Transferred == 0")
	}
	if vol2.Step != system.StepFailed {
		t.Fatalf("vol2 step %s, expected failed", vol2.Step)
	}
	if vol2.Error == "" {
		t.Fatal("vol2 error is empty")
	}
	t.Logf("vol2 error: %s", vol2.Error)

	source1 := app.DockerVolumes[0].Source
	target1 := testutil.SymlinkTarget(t, source1)
	if target1 == "" {
		t.Fatalf("vol1 source is not a symlink")
	}
	if !strings.HasPrefix(target1, driveMount) {
		t.Fatalf("vol1 symlink not on drive: %s", target1)
	}

	source2 := app.DockerVolumes[1].Source
	target2 := testutil.SymlinkTarget(t, source2)
	if target2 != "" {
		t.Fatalf("vol2 source should not be a symlink after failure, got: %s -> %s", source2, target2)
	}

	sourceDir := testutil.VolumeDataRoot(volName2)
	backupDir := sourceDir + ".bak"
	if _, err := os.Stat(backupDir); err == nil {
		if err := os.Rename(backupDir, sourceDir); err != nil {
			t.Logf("warning: failed to restore %s: %v", sourceDir, err)
		}
	}
}
