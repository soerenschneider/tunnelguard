package main

import (
	"testing"
	"time"
)

const testTimeout = 3 * time.Minute

var testNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

func seenAgo(key string, ago time.Duration) Peer {
	hs := testNow.Add(-ago)
	return Peer{PublicKey: key, HandshakeLastSeen: &hs}
}

func neverSeen(key string) Peer {
	return Peer{PublicKey: key}
}

func peerKeys(peers []Peer) []string {
	out := make([]string, 0, len(peers))
	for _, p := range peers {
		out = append(out, p.PublicKey)
	}
	return out
}

func assertKeys(t *testing.T, got []Peer, want []string) {
	t.Helper()
	gotKeys := peerKeys(got)
	if len(gotKeys) != len(want) {
		t.Fatalf("stale peers = %v, want %v", gotKeys, want)
	}
	for i := range want {
		if gotKeys[i] != want[i] {
			t.Fatalf("stale peers = %v, want %v", gotKeys, want)
		}
	}
}

func TestEvaluatePeers(t *testing.T) {
	tests := []struct {
		name       string
		peers      []Peer
		wantStale  []string
		wantMaxAge time.Duration
	}{
		{
			name: "no peers",
		},
		{
			name:       "all fresh",
			peers:      []Peer{seenAgo("a", 10*time.Second), seenAgo("b", time.Minute)},
			wantMaxAge: time.Minute,
		},
		{
			name:       "one stale among fresh",
			peers:      []Peer{seenAgo("a", 10*time.Second), seenAgo("b", 5*time.Minute)},
			wantStale:  []string{"b"},
			wantMaxAge: 5 * time.Minute,
		},
		{
			name:       "exactly at timeout is stale",
			peers:      []Peer{seenAgo("a", testTimeout)},
			wantStale:  []string{"a"},
			wantMaxAge: testTimeout,
		},
		{
			name:       "one tick under timeout is fresh",
			peers:      []Peer{seenAgo("a", testTimeout-time.Nanosecond)},
			wantMaxAge: testTimeout - time.Nanosecond,
		},
		{
			name:       "never-seen peer is maximally stale",
			peers:      []Peer{neverSeen("c")},
			wantStale:  []string{"c"},
			wantMaxAge: testTimeout,
		},
		{
			// regression: the never-seen peer used to be skipped entirely,
			// so maxAge stayed near zero and it was never reset.
			name:       "never-seen peer alongside fresh peers",
			peers:      []Peer{seenAgo("a", time.Second), seenAgo("b", 2*time.Second), neverSeen("c")},
			wantStale:  []string{"c"},
			wantMaxAge: testTimeout,
		},
		{
			name:       "all stale, order preserved",
			peers:      []Peer{seenAgo("a", 10*time.Minute), neverSeen("b"), seenAgo("c", 4*time.Minute)},
			wantStale:  []string{"a", "b", "c"},
			wantMaxAge: 10 * time.Minute,
		},
		{
			name:       "handshake in the future does not go negative",
			peers:      []Peer{seenAgo("a", -time.Minute)},
			wantMaxAge: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := evaluatePeers(tc.peers, testNow, testTimeout)
			assertKeys(t, ev.Stale, tc.wantStale)
			if ev.MaxAge != tc.wantMaxAge {
				t.Errorf("MaxAge = %v, want %v", ev.MaxAge, tc.wantMaxAge)
			}
		})
	}
}

func TestNextWaitSeconds(t *testing.T) {
	tests := []struct {
		name   string
		maxAge time.Duration
		want   float64
	}{
		{"nothing seen yet", 0, testTimeout.Seconds() + 1},
		{"halfway", 90 * time.Second, 91},
		{"at timeout falls back to default", testTimeout, defaultWaitSeconds},
		{"past timeout falls back to default", 10 * testTimeout, defaultWaitSeconds},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextWaitSeconds(tc.maxAge, testTimeout); got != tc.want {
				t.Errorf("nextWaitSeconds(%v) = %v, want %v", tc.maxAge, got, tc.want)
			}
		})
	}
}

func Test_isStaticEndpoint(t *testing.T) {
	type args struct {
		endpoint string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "ipv4",
			args: args{
				endpoint: "1.1.1.1:443",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ipv4, missing port",
			args: args{
				endpoint: "1.1.1.1",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "invalid ipv4",
			args: args{
				endpoint: "999.999.999.999:443",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "ipv6",
			args: args{
				endpoint: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:443",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ipv6, missing port",
			args: args{
				endpoint: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "invalid ipv6",
			args: args{
				endpoint: "[g3::1]:443",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "hostname",
			args: args{
				endpoint: "my-endpoint:443",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "invalid format",
			args: args{
				endpoint: "my-endpoint",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isStaticEndpoint(tt.args.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("isDnsEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isDnsEndpoint() got = %v, want %v", got, tt.want)
			}
		})
	}
}
