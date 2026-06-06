package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/gin-gonic/gin"
	"github.com/pressly/goose/v3"

	_ "github.com/mattn/go-sqlite3"
)

func initHandlerTestDB(t *testing.T) *db.Queries {
	t.Helper()
	conn, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	queries := db.New(conn)
	goose.SetBaseFS(nil)
	goose.SetDialect("sqlite3")
	if err := goose.Up(conn, "../../migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return queries
}

func setupTest(t *testing.T) *db.Queries {
	t.Helper()
	gin.SetMode(gin.TestMode)
	queries := initHandlerTestDB(t)
	MigrationManager = system.NewMigrationManager(queries)
	return queries
}

func TestListStatsVolume_NoName(t *testing.T) {
	setupTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/volumes", nil)

	ListStatsVolume(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListStatsVolume_Success(t *testing.T) {
	queries := setupTest(t)
	ctx := context.Background()

	batch, err := queries.CreateStatsBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.CreateStatsVolume(ctx, db.CreateStatsVolumeParams{
		BatchID:       batch.ID,
		VolumeName:    "test-vol",
		ContainerName: "/test-ctr",
		SourcePath:    "/data",
		TotalBytes:    1024,
		DurationMs:    10,
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/volumes?name=test-vol", nil)

	ListStatsVolume(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestListStatsVolume_NotFound(t *testing.T) {
	setupTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/volumes?name=nonexistent", nil)

	ListStatsVolume(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "[]\n" && w.Body.String() != "[]" {
		t.Logf("body = %s", w.Body.String())
	}
}

func TestListStatsDrive_NoMountpoint(t *testing.T) {
	setupTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/drives", nil)

	ListStatsDrive(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListStatsDrive_Success(t *testing.T) {
	queries := setupTest(t)
	ctx := context.Background()

	batch, err := queries.CreateStatsBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.CreateStatsDrive(ctx, db.CreateStatsDriveParams{
		BatchID:    batch.ID,
		Mountpoint: "/mnt/data",
		Device:     "/dev/sda1",
		TotalBytes: 1000000,
		UsedBytes:  500000,
		FreeBytes:  500000,
		DurationMs: 5,
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/drives?mountpoint=/mnt/data", nil)

	ListStatsDrive(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestListStatsApplication_NoName(t *testing.T) {
	setupTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/applications", nil)

	ListStatsApplication(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListStatsApplication_Success(t *testing.T) {
	queries := setupTest(t)
	ctx := context.Background()

	batch, err := queries.CreateStatsBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.CreateStatsVolume(ctx, db.CreateStatsVolumeParams{
		BatchID:       batch.ID,
		VolumeName:    "vol-a",
		ContainerName: "/my-app",
		SourcePath:    "/data/a",
		TotalBytes:    2048,
		DurationMs:    10,
	}); err != nil {
		t.Fatal(err)
	}
	if err := queries.CreateStatsVolume(ctx, db.CreateStatsVolumeParams{
		BatchID:       batch.ID,
		VolumeName:    "vol-b",
		ContainerName: "/my-app",
		SourcePath:    "/data/b",
		TotalBytes:    4096,
		DurationMs:    10,
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/applications?name=/my-app", nil)

	ListStatsApplication(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestParseTimeRange_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/volumes?name=test", nil)

	from, to, ok := parseTimeRange(c)
	if !ok {
		t.Fatal("parseTimeRange returned ok=false")
	}
	if !to.After(from) {
		t.Error("expected to > from")
	}
	if to.Sub(from) > 8*24*time.Hour {
		t.Errorf("expected ~7 day range, got %v", to.Sub(from))
	}
}

func TestParseTimeRange_Custom(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/stats/volumes?name=test&from=2026-01-01T00:00:00Z&to=2026-06-01T00:00:00Z", nil)

	from, to, ok := parseTimeRange(c)
	if !ok {
		t.Fatal("parseTimeRange returned ok=false")
	}
	if from.Year() != 2026 || from.Month() != 1 || from.Day() != 1 {
		t.Errorf("from = %v, want 2026-01-01", from)
	}
	if to.Year() != 2026 || to.Month() != 6 || to.Day() != 1 {
		t.Errorf("to = %v, want 2026-06-01", to)
	}
}
