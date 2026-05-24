package main

import (
	"context"
	"database/sql"
	"log"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/pressly/goose/v3"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	conn    *sql.DB
	queries *db.Queries
}

func InitDatabase() *Database {
	conn, err := sql.Open("sqlite3", "./dokvol.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := conn.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	queries := db.New(conn)

	goose.SetBaseFS(nil)
	goose.SetDialect("sqlite3")

	if err := goose.Up(conn, "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	d := &Database{
		conn:    conn,
		queries: queries,
	}

	return d
}

func (d *Database) SaveDrives(ctx context.Context, drives []system.DriveInfo) error {
	for _, dr := range drives {
		_, err := d.queries.CreateDrive(ctx, db.CreateDriveParams{
			Device:     dr.Device,
			Mountpoint: dr.Mountpoint,
			Fstype:     dr.Fstype,
			TotalGb:    int64(dr.TotalGB),
			FreeGb:     int64(dr.FreeGB),
			UsedPct:    dr.UsedPct,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) SaveVolumes(ctx context.Context, volumes []system.VolumeDetail) error {
	for _, v := range volumes {
		_, err := d.queries.CreateVolume(ctx, db.CreateVolumeParams{
			ContainerName: v.ContainerName,
			Type:          v.Type,
			Source:        v.Source,
			Destination:   v.Destination,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) Close() error {
	return d.conn.Close()
}
