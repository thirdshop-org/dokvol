package system

import (
	"context"
	"log"
	"sync"
	"time"

	"dokvol/api/internal/db"

	"github.com/moby/moby/client"
	"github.com/shirou/gopsutil/v4/disk"
)

type StatsCollector struct {
	db     *db.Queries
	docker *client.Client
	mu     sync.Mutex
	ticker *time.Ticker
	stopCh chan struct{}
}

func NewStatsCollector(queries *db.Queries) *StatsCollector {
	docker, err := client.New(client.FromEnv)
	if err != nil {
		log.Printf("stats: cannot create Docker client: %s", err)
	}
	return &StatsCollector{
		db:     queries,
		docker: docker,
		stopCh: make(chan struct{}),
	}
}

func (c *StatsCollector) Start() {
	interval := c.readInterval()

	log.Printf("stats collector: starting with interval %s", interval)
	c.Collect()

	c.ticker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-c.ticker.C:
				interval := c.readInterval()
				c.ticker.Reset(interval)
				c.Collect()
			case <-c.stopCh:
				c.ticker.Stop()
				return
			}
		}
	}()
}

func (c *StatsCollector) Stop() {
	close(c.stopCh)
}

func (c *StatsCollector) Collect() {
	if !c.mu.TryLock() {
		log.Println("stats collector: skipped — previous collection still running")
		return
	}
	defer c.mu.Unlock()

	ctx := context.Background()

	batch, err := c.db.CreateStatsBatch(ctx)
	if err != nil {
		log.Printf("stats collector: failed to create batch: %s", err)
		return
	}

	c.collectVolumes(ctx, batch.ID)
	c.collectDrives(ctx, batch.ID)
	c.pruneOldStats(ctx)
}

func (c *StatsCollector) collectVolumes(ctx context.Context, batchID int64) {
	volumes, err := GetDockerVolumesByContainers()
	if err != nil {
		log.Printf("stats collector: failed to list volumes: %s", err)
		return
	}

	for _, app := range volumes {
		for _, vol := range app.Volumes {
			start := time.Now()
			size, err := dirSize(vol.Source)
			duration := time.Since(start).Milliseconds()
			if err != nil {
				log.Printf("stats collector: dirSize(%s): %s", vol.Source, err)
				continue
			}

			volName := vol.Name
			if volName == "" {
				volName = vol.Source
			}

			if err := c.db.CreateStatsVolume(ctx, db.CreateStatsVolumeParams{
				BatchID:       batchID,
				VolumeName:    volName,
				ContainerName: app.ContainerName,
				SourcePath:    vol.Source,
				TotalBytes:    size,
				DurationMs:    duration,
			}); err != nil {
				log.Printf("stats collector: insert volume %s: %s", volName, err)
			}
		}
	}
}

func (c *StatsCollector) collectDrives(ctx context.Context, batchID int64) {
	drives := GetDrives()

	for _, d := range drives {
		start := time.Now()

		usage, err := disk.Usage(d.Mountpoint)
		duration := time.Since(start).Milliseconds()
		if err != nil {
			log.Printf("stats collector: disk usage for %s: %s", d.Mountpoint, err)
			continue
		}

		if err := c.db.CreateStatsDrive(ctx, db.CreateStatsDriveParams{
			BatchID:    batchID,
			Mountpoint: d.Mountpoint,
			Device:     d.Device,
			TotalBytes: int64(usage.Total),
			UsedBytes:  int64(usage.Used),
			FreeBytes:  int64(usage.Free),
			DurationMs: duration,
		}); err != nil {
			log.Printf("stats collector: insert drive %s: %s", d.Mountpoint, err)
		}
	}
}

func (c *StatsCollector) pruneOldStats(ctx context.Context) {
	pref, err := c.db.GetPreference(ctx, "stats_retention_days")
	if err != nil {
		return
	}

	days := pref.Value
	if days == "" || days == "0" {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -atoi(days))

	if err := c.db.DeleteOldStatsVolume(ctx, cutoff); err != nil {
		log.Printf("stats collector: prune volume stats: %s", err)
	}
	if err := c.db.DeleteOldStatsDrive(ctx, cutoff); err != nil {
		log.Printf("stats collector: prune drive stats: %s", err)
	}

	log.Printf("stats collector: pruned stats older than %s", cutoff.Format("2006-01-02"))
}

func (c *StatsCollector) readInterval() time.Duration {
	pref, err := c.db.GetPreference(context.Background(), "stats_interval_seconds")
	if err != nil || pref.Value == "" {
		return 10 * time.Minute
	}
	seconds := atoi(pref.Value)
	if seconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(seconds) * time.Second
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
