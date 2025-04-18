package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

var (
	flagPrintVersion bool
	flagConfigFile   string
	flagDebug        bool

	BuildVersion string
	CommitHash   string
	GoVersion    string
)

func main() {
	parseFlags()

	if flagPrintVersion {
		fmt.Println(fmt.Sprintf("%s %s go%s", BuildVersion, CommitHash, GoVersion))
		os.Exit(0)
	}

	setupLogger(flagDebug)
	slog.Info("Starting tunnelguard", "version", BuildVersion, "go", GoVersion)

	config, err := readConfig(flagConfigFile)
	if err != nil {
		log.Fatal("could not read config: ", err)
	}

	wgDriver, err := NewWgCli(config.Interface, config.ConfigFile)
	if err != nil {
		slog.Error("could not build wg driver", "err", err)
		os.Exit(1)
	}

	metricsWriter, err := buildMetricsWriter(config)
	if err != nil {
		slog.Error("could not build metrics writer", "err", err)
		os.Exit(1)
	}

	tunnelguard := Tunnelguard{
		wg:            wgDriver,
		metricsWriter: metricsWriter,
		niceNames:     config.PublicKeyDict,
	}

	ctx, cancel := context.WithCancel(context.Background())
	wait := &sync.WaitGroup{}
	wait.Add(1)
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT)

		<-sig
		slog.Info("Got signal, quitting")
		cancel()
	}()

	tunnelguard.Loop(ctx, wait)
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
	flag.StringVar(&flagConfigFile, "config", "", "Path of config file")
	flag.BoolVar(&flagPrintVersion, "version", false, "Print version and exit")
	flag.BoolVar(&flagDebug, "debug", false, "Print debug logs")
	flag.Parse()
}

func buildMetricsWriter(config *TunnelguardConfig) (*MetricsWriter, error) {
	if config.MetricsFile == "" {
		return nil, nil
	}

	basePath := filepath.Dir(config.MetricsFile)
	_, err := os.Stat(basePath)

	if err != nil && os.IsNotExist(err) {
		isUsingDefaultValue := config.MetricsFile == defaultMetricsFile
		if isUsingDefaultValue {
			slog.Warn("Disabling metrics writer, path does not exist", "path", basePath)
		} else {
			return nil, fmt.Errorf("base path for writing metrics does not exist: %w", err)
		}
	}

	return NewMetricsWriter(config.MetricsFile)
}
