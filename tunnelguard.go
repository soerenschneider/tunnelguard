package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

type Tunnelguard struct {
	wg   WireguardDriver
	once sync.Once
}

func (t *Tunnelguard) Loop(ctx context.Context, wg *sync.WaitGroup) {
	t.once.Do(func() {
		defer wg.Done()

		maxHandshakeAge := t.conditionallyResetPeers()
		delay := time.Second * time.Duration(maxHandshakeAge)

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
				maxHandshakeAge := t.conditionallyResetPeers()
				delay = time.Second * time.Duration(maxHandshakeAge)
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

func (t *Tunnelguard) conditionallyResetPeers() float64 {
	MetricHeartBeat.SetToCurrentTime()
	peers, err := t.wg.GetPeers()
	if err != nil {
		slog.Error("can't get WireGuard peers", "error", err)
		t.conditionallyFixTunnel()
		labels := prometheus.Labels{
			"error": "get_peers",
		}
		MetricErrorsTotal.With(labels).Inc()
		return defaultWaitSeconds
	}

	var maxHandshakeAge float64 = 0
	for _, peer := range peers {
		hasLastSeen := peer.HandshakeLastSeen != nil

		if hasLastSeen {
			timeSinceHandshake := time.Since(*peer.HandshakeLastSeen)
			slog.Debug("time since latest handshake", "latest_handshake", timeSinceHandshake, "peer", peer.PublicKey)
			if timeSinceHandshake.Seconds() > maxHandshakeAge {
				maxHandshakeAge = timeSinceHandshake.Seconds()
			}
			MetricLatestHandshakeTimestamp.WithLabelValues(peer.PublicKey).Set(float64(peer.HandshakeLastSeen.Unix()))
		}

		if hasLastSeen && time.Since(*peer.HandshakeLastSeen) >= handshakeTimeout {
			t.resetPeer(peer)
		}
	}

	if handshakeTimeout.Seconds()-maxHandshakeAge <= 0 {
		return defaultWaitSeconds
	}
	return (handshakeTimeout.Seconds() - maxHandshakeAge) + 1
}

func (t *Tunnelguard) resetPeer(peer Peer) {
	endpoint, err := t.wg.GetEndpoint(peer.PublicKey)
	if err != nil {
		labels := prometheus.Labels{
			"error": "get_endpoint",
		}
		MetricErrorsTotal.With(labels).Inc()
		slog.Error("could not get endpoint", "pub_key", peer.PublicKey)

		t.conditionallyFixTunnel()
		return
	}

	if len(endpoint) == 0 {
		return
	}

	endpointIsStatic, _ := isStaticEndpoint(endpoint)
	if endpointIsStatic {
		slog.Info("not resetting peer, endpoint is static", "endpoint", endpoint, "pub_key", peer.PublicKey)
		return
	}

	MetricPeerResets.WithLabelValues(peer.PublicKey).Inc()
	slog.Info("resetting peer", "endpoint", endpoint, "pub_key", peer.PublicKey)
	if err := t.wg.ResetPeer(peer.PublicKey, endpoint); err != nil {
		slog.Error("failed to reset peer", "error", err)
		labels := prometheus.Labels{
			"error": "reset_peer",
		}
		MetricErrorsTotal.With(labels).Inc()

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
