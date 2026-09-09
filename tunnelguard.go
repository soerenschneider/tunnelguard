package main

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	handshakeTimeout   = 180 * time.Second
	defaultWaitSeconds = 30
)

var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

type WireguardDriver interface {
	GetPeers() (map[string]Peer, error)
	ResetPeer(publicKey string, endpoint string) error
	AttachEndpoints(peers map[string]Peer) (map[string]Peer, error)
	StartTunnel() error
	IsTunnelUp() (bool, error)
}

type WgConfig struct {
	Peers []Peer `toml:"Peer"`
}

type Peer struct {
	PublicKey         string
	HandshakeLastSeen *time.Time
	Endpoint          string
}

type peerEvaluation struct {
	Stale  []Peer
	MaxAge time.Duration
}

type Tunnelguard struct {
	wg            WireguardDriver
	peerNames     map[string]string
	once          sync.Once
	metricsWriter *MetricsWriter
}

func (t *Tunnelguard) Loop(ctx context.Context, wg *sync.WaitGroup) {
	t.once.Do(func() {
		defer wg.Done()

		maxHandshakeAge := t.conditionallyResetPeers()
		delay := time.Second * time.Duration(maxHandshakeAge)
		silenceMetricsWriterWarnLogs := false

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
				maxHandshakeAge := t.conditionallyResetPeers()
				delay = time.Second * time.Duration(maxHandshakeAge)

				if t.metricsWriter != nil {
					if err := t.metricsWriter.Dump(); err != nil && !silenceMetricsWriterWarnLogs {
						silenceMetricsWriterWarnLogs = true
						slog.Warn("can not write metrics data", "err", err)
					} else {
						silenceMetricsWriterWarnLogs = false
					}
				}
			}
		}
	})
}

func (t *Tunnelguard) conditionallyFixTunnel() {
	connected, err := t.wg.IsTunnelUp()
	if err != nil {
		slog.Error("error while checking if tunnel is up", "error", err)
	} else if connected {
		return
	}

	slog.Warn("Tunnel appears to be down, trying to start tunnel")
	if err := t.wg.StartTunnel(); err != nil {
		slog.Error("starting tunnel failed", "error", err)
	}
}

func getPeersToReset(peers map[string]Peer, now time.Time, timeout time.Duration) peerEvaluation {
	var ev peerEvaluation

	for _, peer := range peers {
		if !isResettableEndpoint(peer.Endpoint) {
			slog.Debug("peer is not resettable", "pub_key", peer.PublicKey)
			continue
		}

		age := timeout // never handshaked == maximally stale
		if peer.HandshakeLastSeen != nil {
			age = now.Sub(*peer.HandshakeLastSeen)
		}
		if age > ev.MaxAge {
			ev.MaxAge = age
		}
		if age >= timeout {
			ev.Stale = append(ev.Stale, peer)
		}
	}

	return ev
}

func nextWaitSeconds(maxAge, timeout time.Duration) float64 {
	if wait := timeout - maxAge; wait > 0 {
		return wait.Seconds() + 1
	}
	return defaultWaitSeconds
}

func (t *Tunnelguard) conditionallyResetPeers() float64 {
	metrics.Heartbeat = time.Now().Unix()

	peers, err := t.wg.GetPeers()
	if err != nil {
		slog.Error("can't get WireGuard peers", "error", err)
		t.conditionallyFixTunnel()
		metrics.ErrorsTotal["get_peers"]++
		return defaultWaitSeconds
	}

	peers, err = t.wg.AttachEndpoints(peers)
	if err != nil {
		metrics.ErrorsTotal["get_endpoint"]++
		slog.Error("could not get endpoints", "err", err)

		t.conditionallyFixTunnel()
		return defaultWaitSeconds
	}

	stalePeers := getPeersToReset(peers, time.Now(), handshakeTimeout)
	for _, peer := range stalePeers.Stale {
		slog.Info("resetting stale peer", "peer", peer.PublicKey, "never_seen", peer.HandshakeLastSeen == nil)
		t.resetPeer(peer)
	}

	t.exportHandshakeMetrics(peers)

	return nextWaitSeconds(stalePeers.MaxAge, handshakeTimeout)
}

func (t *Tunnelguard) exportHandshakeMetrics(peers map[string]Peer) {
	for _, peer := range peers {
		var ts int64
		if peer.HandshakeLastSeen != nil {
			ts = peer.HandshakeLastSeen.Unix()
		}

		if metrics.LatestHandshakeTimestamp[peer.PublicKey] == nil {
			metrics.LatestHandshakeTimestamp[peer.PublicKey] = &peerMetricValue{}
		}
		metrics.LatestHandshakeTimestamp[peer.PublicKey].Value = ts
		metrics.LatestHandshakeTimestamp[peer.PublicKey].PeerName = t.peerNames[peer.PublicKey]
		metrics.LatestHandshakeTimestamp[peer.PublicKey].Resettable = isResettableEndpoint(peer.Endpoint)
	}
}

func (t *Tunnelguard) resetPeer(peer Peer) {
	if metrics.PeerResets[peer.PublicKey] == nil {
		metrics.PeerResets[peer.PublicKey] = &peerMetricValue{}
	}
	metrics.PeerResets[peer.PublicKey].Value = metrics.PeerResets[peer.PublicKey].Value + 1
	metrics.PeerResets[peer.PublicKey].PeerName = t.peerNames[peer.PublicKey]
	slog.Info("resetting peer", "endpoint", peer.Endpoint, "pub_key", peer.PublicKey)
	if err := t.wg.ResetPeer(peer.PublicKey, peer.Endpoint); err != nil {
		slog.Error("failed to reset peer", "error", err)
		metrics.ErrorsTotal["reset_peer"]++
		t.conditionallyFixTunnel()
	}
}

// isResettableEndpoint reports whether a peer's endpoint is worth re-applying.
// Only hostnames are: re-resolving a literal address can't yield anything new,
// so resetting such a peer would just re-pin it to the same IP.
func isResettableEndpoint(endpoint string) bool {
	if strings.TrimSpace(endpoint) == "" {
		return false
	}

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" || port == "" {
		return false
	}

	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return false
	}

	// A literal IP (v4, v6, or zoned v6) has nothing to re-resolve.
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}

	return hostnameRegex.MatchString(host)
}
