package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"hpad-app/internal/devicelogs"
)

func main() {
	cfg, listOnly, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if listOnly {
		if err := devicelogs.List(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := devicelogs.Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (devicelogs.Config, bool, error) {
	fs := flag.NewFlagSet("hpad-log", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := devicelogs.DefaultConfig()
	var listOnly bool
	var padOnly bool
	var dongleOnly bool

	fs.BoolVar(&listOnly, "list", false, "list discovered HPAD serial log devices and exit")
	fs.BoolVar(&cfg.Timestamps, "timestamps", false, "prefix log lines with host timestamps")
	fs.BoolVar(&padOnly, "pad-only", false, "only monitor the HPAD macropad")
	fs.BoolVar(&dongleOnly, "dongle-only", false, "only monitor the HPAD dongle")
	fs.BoolVar(&cfg.Debug, "debug", false, "print additional discovery diagnostics while scanning")

	if err := fs.Parse(args); err != nil {
		return devicelogs.Config{}, false, err
	}
	if padOnly && dongleOnly {
		return devicelogs.Config{}, false, fmt.Errorf("cannot combine --pad-only and --dongle-only")
	}

	switch {
	case padOnly:
		cfg.IncludeDongle = false
	case dongleOnly:
		cfg.IncludePad = false
	}

	return cfg, listOnly, nil
}
