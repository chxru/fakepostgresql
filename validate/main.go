package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/tracelog"
)

func main() {
	slog.Info("Trying to connect with db")

	config, err := pgx.ParseConfig("postgres://localhost:6769/db")
	if err != nil {
		slog.Error("couldn't parse config", "error", err)
	}

	config.Tracer = &tracelog.TraceLog{
		Logger: tracelog.LoggerFunc(func(
			ctx context.Context,
			level tracelog.LogLevel,
			msg string,
			data map[string]any,
		) {
			fmt.Printf("pgx %s: %s %v\n", level, msg, data)
		}),
		LogLevel: tracelog.LogLevelTrace,
	}

	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		slog.Error("Could not initiate the connection with postgres instance", "error", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	// simple query
	_, err = conn.Exec(context.Background(), "select 1")
	if err != nil {
		slog.Error("select 1 failed", "error", err)
	}

	// extended query
	_, err = conn.Exec(context.Background(),
		"select id, name, city as location from users where id = 4",
		pgx.QueryExecModeExec,
		int32(1),
	)
	if err != nil {
		slog.Error("extended query failed", "error", err)
	}

}
