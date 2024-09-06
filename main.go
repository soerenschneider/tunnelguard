package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	opts             = Options{}
	flagPrintVersion bool
)

type Options struct {
	Interface  string
	ConfigFile string
	Debug      bool

	MetricsAddr string
	MetricsDir  string
}

func main() {
	parseFlags()

	if flagPrintVersion {
		fmt.Println(BuildVersion)
		os.Exit(0)
	}

	setupLogger(opts.Debug)
	slog.Info("Starting tunnelguard", "version", BuildVersion)

	wgDriver, err := NewWgCli(opts.Interface, opts.ConfigFile)
	if err != nil {
		slog.Error("could not build wg driver", "error", err)
		os.Exit(1)
	}

	tunnelguard := Tunnelguard{
		wg: wgDriver,
	}

	ctx, cancel := context.WithCancel(context.Background())
	wait := &sync.WaitGroup{}
	wait.Add(1)
	go func() {
		tunnelguard.Loop(ctx, wait)
	}()

	go func() {
		if len(opts.MetricsAddr) > 0 {
			if err := StartMetricsServer(opts.MetricsAddr); err != nil {
				slog.Error("could not start metrics server", "error", err)
				os.Exit(1)
			}
		} else if len(opts.MetricsDir) > 0 {
			if err := StartWritingMetrics(ctx, opts.MetricsDir); err != nil {
				slog.Error("could not start metrics writer", "error", err)
				os.Exit(1)
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)

	<-sig
	slog.Info("Got signal, quitting")
	cancel()
	wait.Wait()
}

func setupLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(logHandler)
	slog.SetDefault(logger)
}

func parseFlags() {
	flag.BoolVar(&flagPrintVersion, "version", false, "Print version and exit")
	flag.BoolVar(&opts.Debug, "debug", false, "Print debug logs")
	flag.StringVar(&opts.Interface, "int", "wg0", "WireGuard interface")
	flag.StringVar(&opts.ConfigFile, "config", "/etc/wireguard/wg0.conf", "WireGuard config file")
	flag.StringVar(&opts.MetricsAddr, "metrics-addr", "", "Start metrics server")
	flag.StringVar(&opts.MetricsDir, "metrics-dir", "/var/lib/node_exporter", "Dir to write metrics to")
	flag.Parse()
}
