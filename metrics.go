package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
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
tunnelguard_peers_resets_total{pub_key="{{ $key }}",peer_name="{{ $value.PeerName }}"} {{ $value.Value }}
{{- end }}
{{- end }}
{{- if gt (len .LatestHandshakeTimestamp) 0 }}
# HELP tunnelguard_peers_latest_handshake_timestap_seconds the timestamp of a peer's most recent handshake
# TYPE tunnelguard_peers_latest_handshake_timestap_seconds gauge
{{- range $key, $value := .LatestHandshakeTimestamp }}
tunnelguard_peers_latest_handshake_timestap_seconds{pub_key="{{ $key }}",resettable="{{ $value.Resettable }}",peer_name="{{ $value.PeerName }}"} {{ $value.Value }}
{{- end }}
{{- end }}
{{- with .Runtime }}
# HELP go_info Information about the Go environment.
# TYPE go_info gauge
go_info{app="tunnelguard",version="{{ .GoVersion }}"} 1
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines{app="tunnelguard"} {{ .Goroutines }}
# HELP go_threads Number of OS threads created.
# TYPE go_threads gauge
go_threads{app="tunnelguard"} {{ .Threads }}
# HELP go_gc_duration_seconds A summary of the wall-time pause (interruption) duration in garbage collection cycles.
# TYPE go_gc_duration_seconds summary
{{- range .GCQuantiles }}
go_gc_duration_seconds{app="tunnelguard",quantile="{{ .Quantile }}"} {{ .Value }}
{{- end }}
go_gc_duration_seconds_sum{app="tunnelguard"} {{ .GCPauseSum }}
go_gc_duration_seconds_count{app="tunnelguard"} {{ .GCCount }}
# HELP go_memstats_alloc_bytes Number of bytes allocated in heap and currently in use.
# TYPE go_memstats_alloc_bytes gauge
go_memstats_alloc_bytes{app="tunnelguard"} {{ .HeapAlloc }}
# HELP go_memstats_alloc_bytes_total Total number of bytes allocated in heap until now.
# TYPE go_memstats_alloc_bytes_total counter
go_memstats_alloc_bytes_total{app="tunnelguard"} {{ .TotalAlloc }}
# HELP go_memstats_mallocs_total Total number of heap objects allocated.
# TYPE go_memstats_mallocs_total counter
go_memstats_mallocs_total{app="tunnelguard"} {{ .Mallocs }}
# HELP go_memstats_frees_total Total number of heap objects frees.
# TYPE go_memstats_frees_total counter
go_memstats_frees_total{app="tunnelguard"} {{ .Frees }}
# HELP go_memstats_sys_bytes Number of bytes obtained from system.
# TYPE go_memstats_sys_bytes gauge
go_memstats_sys_bytes{app="tunnelguard"} {{ .Sys }}
# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and currently in use.
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes{app="tunnelguard"} {{ .HeapAlloc }}
# HELP go_memstats_heap_sys_bytes Number of heap bytes obtained from system.
# TYPE go_memstats_heap_sys_bytes gauge
go_memstats_heap_sys_bytes{app="tunnelguard"} {{ .HeapSys }}
# HELP go_memstats_heap_idle_bytes Number of heap bytes waiting to be used.
# TYPE go_memstats_heap_idle_bytes gauge
go_memstats_heap_idle_bytes{app="tunnelguard"} {{ .HeapIdle }}
# HELP go_memstats_heap_inuse_bytes Number of heap bytes that are in use.
# TYPE go_memstats_heap_inuse_bytes gauge
go_memstats_heap_inuse_bytes{app="tunnelguard"} {{ .HeapInuse }}
# HELP go_memstats_heap_released_bytes Number of heap bytes released to OS.
# TYPE go_memstats_heap_released_bytes gauge
go_memstats_heap_released_bytes{app="tunnelguard"} {{ .HeapReleased }}
# HELP go_memstats_heap_objects Number of currently allocated objects.
# TYPE go_memstats_heap_objects gauge
go_memstats_heap_objects{app="tunnelguard"} {{ .HeapObjects }}
# HELP go_memstats_stack_inuse_bytes Number of bytes obtained from system for stack allocator in non-CGO environments.
# TYPE go_memstats_stack_inuse_bytes gauge
go_memstats_stack_inuse_bytes{app="tunnelguard"} {{ .StackInuse }}
# HELP go_memstats_stack_sys_bytes Number of bytes obtained from system for stack allocator.
# TYPE go_memstats_stack_sys_bytes gauge
go_memstats_stack_sys_bytes{app="tunnelguard"} {{ .StackSys }}
# HELP go_memstats_mspan_inuse_bytes Number of bytes in use by mspan structures.
# TYPE go_memstats_mspan_inuse_bytes gauge
go_memstats_mspan_inuse_bytes{app="tunnelguard"} {{ .MSpanInuse }}
# HELP go_memstats_mspan_sys_bytes Number of bytes used for mspan structures obtained from system.
# TYPE go_memstats_mspan_sys_bytes gauge
go_memstats_mspan_sys_bytes{app="tunnelguard"} {{ .MSpanSys }}
# HELP go_memstats_mcache_inuse_bytes Number of bytes in use by mcache structures.
# TYPE go_memstats_mcache_inuse_bytes gauge
go_memstats_mcache_inuse_bytes{app="tunnelguard"} {{ .MCacheInuse }}
# HELP go_memstats_mcache_sys_bytes Number of bytes used for mcache structures obtained from system.
# TYPE go_memstats_mcache_sys_bytes gauge
go_memstats_mcache_sys_bytes{app="tunnelguard"} {{ .MCacheSys }}
# HELP go_memstats_buck_hash_sys_bytes Number of bytes used by the profiling bucket hash table.
# TYPE go_memstats_buck_hash_sys_bytes gauge
go_memstats_buck_hash_sys_bytes{app="tunnelguard"} {{ .BuckHashSys }}
# HELP go_memstats_gc_sys_bytes Number of bytes used for garbage collection system metadata.
# TYPE go_memstats_gc_sys_bytes gauge
go_memstats_gc_sys_bytes{app="tunnelguard"} {{ .GCSys }}
# HELP go_memstats_other_sys_bytes Number of bytes used for other system allocations.
# TYPE go_memstats_other_sys_bytes gauge
go_memstats_other_sys_bytes{app="tunnelguard"} {{ .OtherSys }}
# HELP go_memstats_next_gc_bytes Number of heap bytes when next garbage collection will take place.
# TYPE go_memstats_next_gc_bytes gauge
go_memstats_next_gc_bytes{app="tunnelguard"} {{ .NextGC }}
# HELP go_memstats_last_gc_time_seconds Number of seconds since 1970 of last garbage collection.
# TYPE go_memstats_last_gc_time_seconds gauge
go_memstats_last_gc_time_seconds{app="tunnelguard"} {{ .LastGCSeconds }}
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds{app="tunnelguard"} {{ .StartTimeSeconds }}
{{- if .HaveProcStat }}
# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.
# TYPE process_cpu_seconds_total counter
process_cpu_seconds_total{app="tunnelguard"} {{ .CPUSeconds }}
{{- end }}
{{- if .HaveProcStatm }}
# HELP process_virtual_memory_bytes Virtual memory size in bytes.
# TYPE process_virtual_memory_bytes gauge
process_virtual_memory_bytes{app="tunnelguard"} {{ .VirtualMemoryBytes }}
# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes{app="tunnelguard"} {{ .ResidentMemoryBytes }}
{{- end }}
{{- if .HaveOpenFDs }}
# HELP process_open_fds Number of open file descriptors.
# TYPE process_open_fds gauge
process_open_fds{app="tunnelguard"} {{ .OpenFDs }}
{{- end }}
{{- end }}
`

var tmpl = template.Must(template.New("metrics").Parse(templateData))

// processStartTime is captured once at startup; unlike the other runtime
// values it must not be re-sampled per scrape.
var processStartTime = formatValue(float64(time.Now().UnixNano()) / 1e9)

// formatValue renders a sample value the way prometheus/common/expfmt does:
// strconv.AppendFloat with 'g' and -1 precision. That means large values come
// out in exponent form (2.023936e+06), matching reference exporter output.
func formatValue(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func formatUint(u uint64) string {
	return formatValue(float64(u))
}

func formatInt(i int) string {
	return formatValue(float64(i))
}

type peerMetricValue struct {
	Value      int64
	PeerName   string
	Resettable bool
}

type gcQuantile struct {
	Quantile string
	Value    string
}

// runtimeMetrics mirrors the metric set exposed by client_golang's
// collectors.NewGoCollector() and NewProcessCollector(). Every sample value is
// a pre-formatted string, since the Prometheus data model has no integer type
// and text/template would otherwise apply its own %v formatting.
type runtimeMetrics struct {
	GoVersion  string
	Goroutines string
	Threads    string

	GCQuantiles []gcQuantile
	GCPauseSum  string
	GCCount     string

	HeapAlloc     string
	TotalAlloc    string
	Mallocs       string
	Frees         string
	Sys           string
	HeapSys       string
	HeapIdle      string
	HeapInuse     string
	HeapReleased  string
	HeapObjects   string
	StackInuse    string
	StackSys      string
	MSpanInuse    string
	MSpanSys      string
	MCacheInuse   string
	MCacheSys     string
	BuckHashSys   string
	GCSys         string
	OtherSys      string
	NextGC        string
	LastGCSeconds string

	StartTimeSeconds    string
	CPUSeconds          string
	VirtualMemoryBytes  string
	ResidentMemoryBytes string
	OpenFDs             string

	HaveProcStat  bool
	HaveProcStatm bool
	HaveOpenFDs   bool
}

type Metrics struct {
	Version                  map[string]string
	Heartbeat                int64
	LastStatusChange         int64
	ErrorsTotal              map[string]int64
	PeerResets               map[string]*peerMetricValue
	LatestHandshakeTimestamp map[string]*peerMetricValue
	Runtime                  *runtimeMetrics
}

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

// Render writes the exposition text. The runtime block is sampled here rather
// than at init so the values reflect the moment of the scrape.
func (m *Metrics) Render(w io.Writer) error {
	snapshot := *m
	snapshot.Runtime = collectRuntimeMetrics()
	return tmpl.Execute(w, &snapshot)
}

func collectRuntimeMetrics() *runtimeMetrics {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	nThreads, _ := runtime.ThreadCreateProfile(nil)

	rm := &runtimeMetrics{
		GoVersion:  runtime.Version(),
		Goroutines: formatInt(runtime.NumGoroutine()),
		Threads:    formatInt(nThreads),

		GCQuantiles: gcQuantiles(&ms),
		GCPauseSum:  formatValue(float64(ms.PauseTotalNs) / 1e9),
		GCCount:     formatUint(uint64(ms.NumGC)),

		HeapAlloc:     formatUint(ms.HeapAlloc),
		TotalAlloc:    formatUint(ms.TotalAlloc),
		Mallocs:       formatUint(ms.Mallocs),
		Frees:         formatUint(ms.Frees),
		Sys:           formatUint(ms.Sys),
		HeapSys:       formatUint(ms.HeapSys),
		HeapIdle:      formatUint(ms.HeapIdle),
		HeapInuse:     formatUint(ms.HeapInuse),
		HeapReleased:  formatUint(ms.HeapReleased),
		HeapObjects:   formatUint(ms.HeapObjects),
		StackInuse:    formatUint(ms.StackInuse),
		StackSys:      formatUint(ms.StackSys),
		MSpanInuse:    formatUint(ms.MSpanInuse),
		MSpanSys:      formatUint(ms.MSpanSys),
		MCacheInuse:   formatUint(ms.MCacheInuse),
		MCacheSys:     formatUint(ms.MCacheSys),
		BuckHashSys:   formatUint(ms.BuckHashSys),
		GCSys:         formatUint(ms.GCSys),
		OtherSys:      formatUint(ms.OtherSys),
		NextGC:        formatUint(ms.NextGC),
		LastGCSeconds: formatValue(float64(ms.LastGC) / 1e9),

		StartTimeSeconds: processStartTime,
	}

	addProcessMetrics(rm)
	return rm
}

// gcQuantiles derives the 0/25/50/75/100 percentiles from the runtime's
// circular buffer of the most recent 256 GC pauses, matching client_golang.
func gcQuantiles(ms *runtime.MemStats) []gcQuantile {
	n := int(ms.NumGC)
	if n > len(ms.PauseNs) {
		n = len(ms.PauseNs)
	}

	pauses := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		idx := (int(ms.NumGC) + len(ms.PauseNs) - 1 - i) % len(ms.PauseNs)
		pauses = append(pauses, float64(ms.PauseNs[idx])/1e9)
	}
	sort.Float64s(pauses)

	quantiles := []struct {
		label string
		frac  float64
	}{{"0", 0}, {"0.25", 0.25}, {"0.5", 0.5}, {"0.75", 0.75}, {"1", 1}}

	out := make([]gcQuantile, 0, len(quantiles))
	for _, q := range quantiles {
		v := 0.0
		if len(pauses) > 0 {
			v = pauses[int(float64(len(pauses)-1)*q.frac+0.5)]
		}
		out = append(out, gcQuantile{Quantile: q.label, Value: formatValue(v)})
	}
	return out
}

func addProcessMetrics(rm *runtimeMetrics) {
	pageSize := uint64(os.Getpagesize())

	if b, err := os.ReadFile("/proc/self/statm"); err == nil {
		if f := strings.Fields(string(b)); len(f) >= 2 {
			vsz, err1 := strconv.ParseUint(f[0], 10, 64)
			rss, err2 := strconv.ParseUint(f[1], 10, 64)
			if err1 == nil && err2 == nil {
				rm.VirtualMemoryBytes = formatUint(vsz * pageSize)
				rm.ResidentMemoryBytes = formatUint(rss * pageSize)
				rm.HaveProcStatm = true
			}
		}
	}

	if b, err := os.ReadFile("/proc/self/stat"); err == nil {
		// utime/stime are fields 14 and 15, but the comm field can contain
		// spaces and parens, so index from the last ')' rather than splitting
		// the whole line.
		if i := strings.LastIndexByte(string(b), ')'); i >= 0 {
			f := strings.Fields(string(b)[i+1:])
			if len(f) >= 13 {
				utime, err1 := strconv.ParseFloat(f[11], 64)
				stime, err2 := strconv.ParseFloat(f[12], 64)
				if err1 == nil && err2 == nil {
					// Divisor is USER_HZ, 100 on all mainstream Linux builds.
					rm.CPUSeconds = formatValue((utime + stime) / 100)
					rm.HaveProcStat = true
				}
			}
		}
	}

	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		rm.OpenFDs = formatInt(len(entries))
		rm.HaveOpenFDs = true
	}
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
	metrics.Runtime = collectRuntimeMetrics()

	tmpFile := fmt.Sprintf("%s.tmp", m.metricsFile)
	file, err := os.Create(tmpFile)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if err := m.tmpl.Execute(file, metrics); err != nil {
		return fmt.Errorf("could not execute template: %w", err)
	}

	return os.Rename(tmpFile, m.metricsFile)
}
