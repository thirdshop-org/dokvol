package system

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dokvol/api/internal/db"

	"github.com/pressly/goose/v3"

	_ "github.com/mattn/go-sqlite3"
)

func initStatsDB(t *testing.T) (*db.Queries, *sql.DB) {
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
	return queries, conn
}

func batchCount(t *testing.T, conn *sql.DB) int {
	t.Helper()
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM stats_batch").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"600", 600},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"10m", 0},
		{"3600", 3600},
		{"999999", 999999},
	}
	for _, tt := range tests {
		got := atoi(tt.input)
		if got != tt.want {
			t.Errorf("atoi(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), make([]byte, 100), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), make([]byte, 200), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "c.txt"), make([]byte, 300), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize(%s): %v", dir, err)
	}
	if got != 600 {
		t.Errorf("dirSize(%s) = %d, want 600", dir, got)
	}
}

func TestDirSize_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize(%s): %v", dir, err)
	}
	if got != 0 {
		t.Errorf("dirSize(%s) = %d, want 0", dir, got)
	}
}

func TestDirSize_NotFound(t *testing.T) {
	_, err := dirSize("/tmp/does-not-exist-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestStatsCollector_ReadInterval_Default(t *testing.T) {
	queries, _ := initStatsDB(t)
	c := NewStatsCollector(queries)

	got := c.readInterval()
	want := 10 * time.Minute
	if got != want {
		t.Errorf("readInterval() = %v, want %v", got, want)
	}
}

func TestStatsCollector_ReadInterval_Custom(t *testing.T) {
	queries, _ := initStatsDB(t)
	c := NewStatsCollector(queries)

	if err := queries.UpsertPreference(context.Background(), db.UpsertPreferenceParams{
		Key:   "stats_interval_seconds",
		Value: "300",
	}); err != nil {
		t.Fatal(err)
	}

	got := c.readInterval()
	want := 5 * time.Minute
	if got != want {
		t.Errorf("readInterval() = %v, want %v", got, want)
	}
}

func TestStatsCollector_ReadInterval_Invalid(t *testing.T) {
	queries, _ := initStatsDB(t)
	c := NewStatsCollector(queries)

	if err := queries.UpsertPreference(context.Background(), db.UpsertPreferenceParams{
		Key:   "stats_interval_seconds",
		Value: "abc",
	}); err != nil {
		t.Fatal(err)
	}

	got := c.readInterval()
	want := 10 * time.Minute
	if got != want {
		t.Errorf("readInterval() = %v, want %v", got, want)
	}
}

func TestStatsCollector_PruneOldStats(t *testing.T) {
	queries, conn := initStatsDB(t)
	c := NewStatsCollector(queries)
	ctx := context.Background()

	if err := queries.UpsertPreference(ctx, db.UpsertPreferenceParams{
		Key:   "stats_retention_days",
		Value: "30",
	}); err != nil {
		t.Fatal(err)
	}

	_, err := conn.Exec("INSERT INTO stats_batch (captured_at) VALUES (datetime('now', '-60 days'))")
	if err != nil {
		t.Fatal(err)
	}
	var oldBatchID int64
	if err := conn.QueryRow("SELECT id FROM stats_batch ORDER BY id DESC LIMIT 1").Scan(&oldBatchID); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec("INSERT INTO stats_volume (batch_id, volume_name, container_name, source_path, total_bytes, duration_ms, captured_at) VALUES (?, 'old-vol', '/ctr', '/old', 100, 5, datetime('now', '-60 days'))", oldBatchID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec("INSERT INTO stats_drive (batch_id, mountpoint, device, total_bytes, used_bytes, free_bytes, duration_ms, captured_at) VALUES (?, '/old', 'dev', 1000, 500, 500, 5, datetime('now', '-60 days'))", oldBatchID)
	if err != nil {
		t.Fatal(err)
	}

	newBatch, err := queries.CreateStatsBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.CreateStatsVolume(ctx, db.CreateStatsVolumeParams{
		BatchID:       newBatch.ID,
		VolumeName:    "new-vol",
		ContainerName: "/ctr",
		SourcePath:    "/new",
		TotalBytes:    200,
		DurationMs:    3,
	}); err != nil {
		t.Fatal(err)
	}
	if err := queries.CreateStatsDrive(ctx, db.CreateStatsDriveParams{
		BatchID:    newBatch.ID,
		Mountpoint: "/new",
		Device:     "dev",
		TotalBytes: 2000,
		UsedBytes:  1000,
		FreeBytes:  1000,
		DurationMs: 3,
	}); err != nil {
		t.Fatal(err)
	}

	c.pruneOldStats(ctx)

	oldVols, _ := queries.ListStatsVolumeByName(ctx, db.ListStatsVolumeByNameParams{
		VolumeName:   "old-vol",
		CapturedAt:   time.Now().Add(-365 * 24 * time.Hour),
		CapturedAt_2: time.Now().Add(365 * 24 * time.Hour),
	})
	if len(oldVols) != 0 {
		t.Errorf("expected 0 old volume stat after prune, got %d", len(oldVols))
	}

	oldDrives, _ := queries.ListStatsDriveByMountpoint(ctx, db.ListStatsDriveByMountpointParams{
		Mountpoint:   "/old",
		CapturedAt:   time.Now().Add(-365 * 24 * time.Hour),
		CapturedAt_2: time.Now().Add(365 * 24 * time.Hour),
	})
	if len(oldDrives) != 0 {
		t.Errorf("expected 0 old drive stat after prune, got %d", len(oldDrives))
	}

	newVols, _ := queries.ListStatsVolumeByName(ctx, db.ListStatsVolumeByNameParams{
		VolumeName:   "new-vol",
		CapturedAt:   time.Now().Add(-24 * time.Hour),
		CapturedAt_2: time.Now().Add(24 * time.Hour),
	})
	if len(newVols) != 1 {
		t.Errorf("expected 1 new volume stat kept after prune, got %d", len(newVols))
	}

	if n := batchCount(t, conn); n != 2 {
		t.Errorf("expected 2 batches (prune keeps batch rows), got %d", n)
	}
}

func TestStatsCollector_PruneOldStats_Disabled(t *testing.T) {
	queries, _ := initStatsDB(t)
	c := NewStatsCollector(queries)
	ctx := context.Background()

	if err := queries.UpsertPreference(ctx, db.UpsertPreferenceParams{
		Key:   "stats_retention_days",
		Value: "0",
	}); err != nil {
		t.Fatal(err)
	}

	batch, err := queries.CreateStatsBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := queries.CreateStatsVolume(ctx, db.CreateStatsVolumeParams{
		BatchID:       batch.ID,
		VolumeName:    "test-vol",
		ContainerName: "/test-ctr",
		SourcePath:    "/tmp",
		TotalBytes:    100,
		DurationMs:    5,
	}); err != nil {
		t.Fatal(err)
	}

	c.pruneOldStats(ctx)

	vols, _ := queries.ListStatsVolumeByName(ctx, db.ListStatsVolumeByNameParams{
		VolumeName:   "test-vol",
		CapturedAt:   time.Now().Add(-24 * time.Hour),
		CapturedAt_2: time.Now().Add(24 * time.Hour),
	})
	if len(vols) != 1 {
		t.Errorf("expected 1 volume stat (prune disabled), got %d", len(vols))
	}
}

func TestStatsCollector_Collect_CreatesBatch(t *testing.T) {
	queries, conn := initStatsDB(t)
	c := NewStatsCollector(queries)

	c.Collect()

	if n := batchCount(t, conn); n == 0 {
		t.Error("expected at least one batch after Collect()")
	}
}

func TestStatsCollector_Collect_SkipOnBusy(t *testing.T) {
	queries, conn := initStatsDB(t)
	c := NewStatsCollector(queries)

	c.mu.Lock()
	c.Collect()
	c.mu.Unlock()

	if n := batchCount(t, conn); n != 0 {
		t.Errorf("expected 0 batches (skipped), got %d", n)
	}
}
