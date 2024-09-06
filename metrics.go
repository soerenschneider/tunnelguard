package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"golang.org/x/sys/unix"
)

const (
	namespace = "tunnelguard"
)

var (
	MetricHeartBeat = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "heartbeat_timestamp_seconds",
	})

	MetricErrorsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "errors_total",
	}, []string{"error"})

	MetricPeerResets = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "peers",
		Name:      "resets_total",
	}, []string{"pub_key"})

	MetricLatestHandshakeTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "peers",
		Name:      "latest_handshake_timestap_seconds",
	}, []string{"pub_key"})
)

func StartMetricsServer(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := http.Server{
		Addr:              addr,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

var once sync.Once

func StartWritingMetrics(ctx context.Context, dir string) error {
	if err := unix.Access(dir, unix.W_OK); err != nil {
		return errors.New("path not writable")
	}

	once.Do(func() {
		ticker := time.NewTicker(60 * time.Second)

		if err := writeMetrics(dir); err != nil {
			slog.Error("could not write metrics", "error", err)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writeMetrics(dir); err != nil {
					slog.Error("could not write metrics", "error", err)
				}
			}
		}
	})

	return nil
}

func writeMetrics(path string) error {
	path, err := url.JoinPath(path, fmt.Sprintf("%s.prom", namespace))
	if err != nil {
		return err
	}

	metrics, err := dumpMetrics()
	if err != nil {
		return err
	}

	// writing a file is not atomic, write to a file which is ignored by prom and rename it
	tmpPath := fmt.Sprintf("%s.tmp", path)
	err = os.WriteFile(tmpPath, []byte(metrics), 0644) // #nosec: G306
	if err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func dumpMetrics() (string, error) {
	var buf = &bytes.Buffer{}
	fmt := expfmt.NewFormat(expfmt.TypeTextPlain)
	enc := expfmt.NewEncoder(buf, fmt)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return "", err
	}

	for _, f := range families {
		// writing all (implicit) metrics will cause a duplication error with other tools writing metrics
		if strings.HasPrefix(f.GetName(), namespace) {
			if err := enc.Encode(f); err != nil {
				slog.Info("could not encode metric", "error", err, "metric", f.GetName())
			}
		}
	}

	return buf.String(), nil
}
