package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"regexp"
	"sync"
	"time"
)

const (
	handshakeTimeout   = 180 * time.Second
	defaultWaitSeconds = 30
)

var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

type WireguardDriver interface {
	GetPeers() ([]Peer, error)
	ResetPeer(publicKey string, endpoint string) error
	GetEndpoint(publicKey string) (string, error)
	StartTunnel() error
	IsTunnelUp() (bool, error)
}

type WgConfig struct {
	Peers []Peer `toml:"Peer"`
}

type Peer struct {
	PublicKey         string
	HandshakeLastSeen *time.Time
	Endpoint          *string
}

type peerEvaluation struct {
	Stale  []Peer
	MaxAge time.Duration
}

type Tunnelguard struct {
	wg            WireguardDriver
	niceNames     map[string]string
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
	}

	if connected {
		return
	}

	slog.Warn("Tunnel appears to be down, trying to start tunnel")
	if err := t.wg.StartTunnel(); err != nil {
		slog.Error("starting tunnel failed", "error", err)
	}
}

func evaluatePeers(peers []Peer, now time.Time, timeout time.Duration) peerEvaluation {
	var ev peerEvaluation
	for _, peer := range peers {
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

	ev := evaluatePeers(peers, time.Now(), handshakeTimeout)
	for _, peer := range ev.Stale {
		slog.Info("resetting stale peer", "peer", peer.PublicKey, "never_seen", peer.HandshakeLastSeen == nil)
		t.resetPeer(peer)
	}

	t.exportHandshakeMetrics(peers)

	return nextWaitSeconds(ev.MaxAge, handshakeTimeout)
}

func (t *Tunnelguard) exportHandshakeMetrics(peers []Peer) {
	for _, peer := range peers {
		var ts int64
		if peer.HandshakeLastSeen != nil {
			ts = peer.HandshakeLastSeen.Unix()
		}

		if metrics.LatestHandshakeTimestamp[peer.PublicKey] == nil {
			metrics.LatestHandshakeTimestamp[peer.PublicKey] = &peerMetricValue{}
		}
		metrics.LatestHandshakeTimestamp[peer.PublicKey].Value = ts
		metrics.LatestHandshakeTimestamp[peer.PublicKey].NiceName = t.niceNames[peer.PublicKey]
	}
}

func (t *Tunnelguard) resetPeer(peer Peer) {
	endpoint, err := t.wg.GetEndpoint(peer.PublicKey)
	if err != nil {
		metrics.ErrorsTotal["get_endpoint"]++
		slog.Error("could not get endpoint", "pub_key", peer.PublicKey)

		t.conditionallyFixTunnel()
		return
	}

	if len(endpoint) == 0 {
		return
	}

	endpointIsStatic, _ := isStaticEndpoint(endpoint)
	if endpointIsStatic {
		slog.Debug("not resetting peer, endpoint is static", "endpoint", endpoint, "pub_key", peer.PublicKey)
		return
	}

	if metrics.PeerResets[peer.PublicKey] == nil {
		metrics.PeerResets[peer.PublicKey] = &peerMetricValue{}
	}
	metrics.PeerResets[peer.PublicKey].Value = metrics.PeerResets[peer.PublicKey].Value + 1
	metrics.PeerResets[peer.PublicKey].NiceName = t.niceNames[peer.PublicKey]
	slog.Info("resetting peer", "endpoint", endpoint, "pub_key", peer.PublicKey)
	if err := t.wg.ResetPeer(peer.PublicKey, endpoint); err != nil {
		slog.Error("failed to reset peer", "error", err)
		metrics.ErrorsTotal["reset_peer"]++
		t.conditionallyFixTunnel()
	}
}

func isStaticEndpoint(endpoint string) (bool, error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false, err
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return true, nil
		} else if ip.To16() != nil {
			return true, nil
		}
	}

	if hostnameRegex.MatchString(host) {
		return false, nil
	}

	return false, errors.New("unknown format")
}
