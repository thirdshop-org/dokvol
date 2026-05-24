package database

import (
	"context"
	"database/sql"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/pressly/goose/v3"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	conn    *sql.DB
	Queries *db.Queries
}

func Init() *Database {
	conn, err := sql.Open("sqlite3", "./dokvol.db")
	if err != nil {
		panic(err)
	}

	if err := conn.Ping(); err != nil {
		panic(err)
	}

	queries := db.New(conn)

	goose.SetBaseFS(nil)
	goose.SetDialect("sqlite3")

	if err := goose.Up(conn, "migrations"); err != nil {
		panic(err)
	}

	return &Database{
		conn:    conn,
		Queries: queries,
	}
}

func (d *Database) SaveDrives(ctx context.Context, drives []system.DriveInfo) error {
	for _, dr := range drives {
		_, err := d.Queries.CreateDrive(ctx, db.CreateDriveParams{
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
		_, err := d.Queries.CreateVolume(ctx, db.CreateVolumeParams{
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
