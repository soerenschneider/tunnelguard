package main

import (
	"fmt"
	"log"
	"os"
	"text/template"
	"time"
)

const templateData = `# HELP tunnelguard_version version information for the running binary
# TYPE tunnelguard_version gauge
tunnelguard_version{app="{{ index .Version "app" }}",go="{{ index .Version "go" }}"} 1
# HELP tunnelguard_heartbeat_timestamp_seconds the timestamp of the invocation
# TYPE tunnelguard_heartbeat_timestamp_seconds gauge
tunnelguard_heartbeat_timestamp_seconds {{ .Heartbeat }}
{{- if gt (len .ErrorsTotal) 0 }}
# HELP tunnelguard_errors_total Number of errors.
# TYPE tunnelguard_errors_total counter
{{- range $key, $value := .ErrorsTotal }}
tunnelguard_errors_total{error="{{ $key }}"} {{ $value }}
{{- end }}
{{- end }}
{{- if gt (len .PeerResets) 0 }}
# HELP tunnelguard_resets_total Number of SSH restart errors encountered.
# TYPE tunnelguard_resets_total counter
{{- range $key, $value := .PeerResets }}
tunnelguard_peers_resets_total{pub_key="{{ $key }}",nice_name="{{ $value.NiceName }}"} {{ $value.Value }}
{{- end }}
{{- end }}
{{- if gt (len .LatestHandshakeTimestamp) 0 }}
# HELP tunnelguard_peers_latest_handshake_timestap_seconds the timestamp of a peer's most recent handshake
# TYPE tunnelguard_peers_latest_handshake_timestap_seconds gauge
{{- range $key, $value := .LatestHandshakeTimestamp }}
tunnelguard_peers_latest_handshake_timestap_seconds{pub_key="{{ $key }}",nice_name="{{ $value.NiceName }}"} {{ $value.Value }}
{{- end }}
{{- end }}
`

var metrics = Metrics{
	Version: map[string]string{
		"go":  GoVersion,
		"app": BuildVersion,
	},
	Heartbeat:                time.Now().Unix(),
	ErrorsTotal:              make(map[string]int64),
	PeerResets:               make(map[string]*peerMetricValue),
	LatestHandshakeTimestamp: make(map[string]*peerMetricValue),
}

type peerMetricValue struct {
	Value    int64
	NiceName string
}

type Metrics struct {
	Version                  map[string]string
	Heartbeat                int64
	LastStatusChange         int64
	ErrorsTotal              map[string]int64
	PeerResets               map[string]*peerMetricValue
	LatestHandshakeTimestamp map[string]*peerMetricValue
}

type MetricsWriter struct {
	tmpl        *template.Template
	metricsFile string
}

func NewMetricsWriter(metricsFile string) (*MetricsWriter, error) {
	tmpl, err := template.New("metrics").Parse(templateData)
	if err != nil {
		return nil, err
	}

	return &MetricsWriter{
		tmpl:        tmpl,
		metricsFile: metricsFile,
	}, nil
}

func (m *MetricsWriter) Dump() error {
	tmpFile := fmt.Sprintf("%s.tmp", m.metricsFile)
	file, err := os.Create(tmpFile)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer file.Close()

	if err := m.tmpl.Execute(file, metrics); err != nil {
		return fmt.Errorf("could not execute template: %w", err)
	}

	return os.Rename(tmpFile, m.metricsFile)
}
