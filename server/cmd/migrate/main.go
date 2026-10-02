// thinhthan-migrate runs the server/migrations pairs through
// golang-migrate: up | down | version | steps ±N | force V.
//
//	thinhthan-migrate -dir server/migrations -dsn "$DATABASE_URL" up
//
// -dsn falls back to DATABASE_URL. Flags are implementer-local conveniences
// (wave2 IMP-005: no new spec surface).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"thinhthan/internal/durable/schema"
)

func main() {
	var (
		dir  = flag.String("dir", "server/migrations", "migrations directory")
		dsn  = flag.String("dsn", os.Getenv("DATABASE_URL"), "postgres DSN (or DATABASE_URL)")
		step = flag.Int("steps", 0, "limited steps for 'steps'")
	)
	flag.Parse()
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "migrate: -dsn or DATABASE_URL required")
		os.Exit(2)
	}
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "migrate: command required: up|down|version|force <v>")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error
	switch args[0] {
	case "up", "down", "up1", "down1":
		err = schema.Migrate(ctx, *dsn, *dir, args[0])
	case "steps":
		if *step == 0 {
			err = fmt.Errorf("-steps N required")
		} else {
			err = schema.MigrateSteps(ctx, *dsn, *dir, *step)
		}
	case "force":
		if len(args) < 2 {
			err = fmt.Errorf("force requires a version")
		} else {
			var v int
			if _, serr := fmt.Sscanf(args[1], "%d", &v); serr != nil {
				err = serr
			} else {
				err = schema.MigrateForce(ctx, *dsn, *dir, v)
			}
		}
	case "version":
		var v uint
		var dirty bool
		v, dirty, err = schema.MigrateVersion(ctx, *dsn, *dir)
		if err == nil {
			fmt.Printf("version=%d dirty=%v\n", v, dirty)
		}
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}
