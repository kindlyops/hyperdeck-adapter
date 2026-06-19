// Command hyperdeck-adapter runs the virtual HyperDeck adapter, as a system-tray
// application by default or headless with -no-tray.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/clipsource"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/clock"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/config"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/stateprobe"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/tray"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/uia"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/vlchttp"
	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driving/hyperdeck"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

func main() {
	dfltConfig := defaultConfigPath()
	configPath := flag.String("config", dfltConfig, "path to profiles.yaml")
	bind := flag.String("bind", "0.0.0.0:9993", "TCP listen address")
	interval := flag.Duration("poll", time.Second, "lock/reconcile poll interval")
	noTray := flag.Bool("no-tray", false, "run headless: log status instead of showing the system tray")
	checkAX := flag.Bool("check-accessibility", false, "prompt for / verify input permission, then exit (0 granted, 1 not)")
	flag.Parse()

	if *checkAX {
		if injector.RequestAccessibility() {
			slog.Info("input permission granted")
			return
		}
		slog.Error("input permission not granted; enable this binary under the OS input/accessibility settings")
		os.Exit(1)
	}

	// First-run convenience: when relying on the default location, seed it with
	// the bundled example profiles so a bare run works without manual setup. An
	// explicit -config path is left untouched — a missing file there is an error.
	if *configPath == dfltConfig {
		if created, err := config.EnsureDefault(*configPath); err != nil {
			slog.Error("seed default config", "err", err)
			os.Exit(1)
		} else if created {
			slog.Info("wrote default config", "path", *configPath)
		}
	}

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
	if !injector.RequestAccessibility() {
		slog.Warn("input permission not granted; keystrokes will not be delivered until this binary is enabled in the OS input/accessibility settings")
	}

	session := app.NewSession()
	uiaEngine := uia.New()
	controller := controlRouter{
		domain.ControlAPI: vlchttp.New(),
		domain.ControlUIA: uiaEngine,
	}
	deck := app.NewVirtualDeck(session, inj, app.WithController(controller))
	clk := clock.New()

	presenter, run := ui(*noTray, deck)

	lm := app.NewLockManager(session, inj, profiles, presenter,
		func(p domain.Profile) port.ClipSource { return clipsource.New(p) },
		func(p domain.Profile) port.StateProbe { return stateprobe.New(p, uiaEngine) })
	rec := app.NewReconciler(session)

	srv := hyperdeck.NewServer(deck, deck)
	ln, err := net.Listen("tcp", *bind)
	if err != nil {
		slog.Error("listen", "addr", *bind, "err", err)
		os.Exit(1)
	}

	lm.Poll() // lock immediately if a player is already running
	go func() { _ = srv.Serve(ln) }()
	go lm.Run(clk, *interval)
	go rec.Run(clk, *interval)

	slog.Info("hyperdeck-adapter started", "bind", *bind, "profiles", len(profiles), "tray", !*noTray)
	run() // blocks: tray event loop, or wait-for-signal when headless
}

// controlRouter dispatches a transport action to the controller backend for the
// profile's control mode (api -> VLC HTTP, uia -> UI Automation).
type controlRouter map[domain.ControlMode]port.PlayerController

func (r controlRouter) Control(p domain.Profile, w domain.Window, key domain.KeyName) error {
	c, ok := r[p.Control]
	if !ok || c == nil {
		return fmt.Errorf("no controller configured for control mode %q", p.Control)
	}
	return c.Control(p, w, key)
}

// ui returns the status presenter and the blocking run loop for the chosen mode.
func ui(noTray bool, deck *app.VirtualDeck) (port.StatusPresenter, func()) {
	if noTray {
		return logPresenter{}, waitForSignal
	}
	t := tray.New(func() { _ = deck.Rehome() }, func() { os.Exit(0) })
	return t, t.Run
}

func waitForSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")
}

// logPresenter implements port.StatusPresenter for headless (-no-tray) runs.
type logPresenter struct{}

func (logPresenter) Present(l domain.LockState) {
	if l.Locked && l.Profile != nil {
		slog.Info("locked", "profile", l.Profile.ID, "window", l.Window.Title)
	} else {
		slog.Info("unlocked (no player)")
	}
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "profiles.yaml"
	}
	return filepath.Join(dir, "hyperdeck-adapter", "profiles.yaml")
}
