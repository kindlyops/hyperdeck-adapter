// Command hyperdeck-adapter runs the virtual HyperDeck tray application.
package main

import (
	"flag"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/clipsource"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/clock"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/config"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/stateprobe"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/tray"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driving/hyperdeck"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to profiles.yaml")
	bind := flag.String("bind", "0.0.0.0:9993", "TCP listen address")
	interval := flag.Duration("poll", time.Second, "lock/reconcile poll interval")
	flag.Parse()

	profiles, err := config.NewStore(*configPath).Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	inj, err := injector.New()
	if err != nil {
		slog.Error("init injector", "err", err)
		os.Exit(1)
	}

	session := app.NewSession()
	deck := app.NewVirtualDeck(session, inj)

	clk := clock.New()
	t := tray.New(func() { _ = deck.Rehome() }, func() { os.Exit(0) })

	lm := app.NewLockManager(session, inj, profiles, t,
		func(p domain.Profile) port.ClipSource { return clipsource.New(p) },
		func(p domain.Profile) port.StateProbe { return stateprobe.New(p) })
	rec := app.NewReconciler(session)

	srv := hyperdeck.NewServer(deck, deck)
	ln, err := net.Listen("tcp", *bind)
	if err != nil {
		slog.Error("listen", "addr", *bind, "err", err)
		os.Exit(1)
	}

	go func() { _ = srv.Serve(ln) }()
	go lm.Run(clk, *interval)
	go rec.Run(clk, *interval)

	slog.Info("hyperdeck-adapter started", "bind", *bind, "profiles", len(profiles))
	t.Run() // blocks on the tray event loop
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "profiles.yaml"
	}
	return dir + "/hyperdeck-adapter/profiles.yaml"
}
