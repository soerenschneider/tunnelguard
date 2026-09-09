package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeHandshakeProvider struct {
	out []byte
	err error
}

func (f fakeHandshakeProvider) GetHandshakeData() ([]byte, error) {
	return f.out, f.err
}

func TestGetPeers(t *testing.T) {
	const (
		keyA = "g5MCSkWzApMkKxUr4KXJ6bQpgGcJnEUsrU80658tEww="
		keyB = "+M8GnUBdLQSnCdgdCsX8ufS8H+6pw/IlG+L+0IGGjGc="
		keyC = "Fpmri35N3j6xe44PX7DI/YWNF0mLCL/FF7jWXmeiDl8="
	)

	tests := []struct {
		name    string
		out     string
		err     error
		want    map[string]int64 // public key -> handshake unix time; 0 means never
		wantErr bool
	}{
		{
			name: "three peers, one never handshaked",
			out: keyA + "\t1788522801\n" +
				keyB + "\t1788522878\n" +
				keyC + "\t0\n",
			want: map[string]int64{
				keyA: 1788522801,
				keyB: 1788522878,
				keyC: 0,
			},
		},
		{
			name: "no trailing newline",
			out:  "aaa=\t1788522801",
			want: map[string]int64{"aaa=": 1788522801},
		},
		{
			name: "blank lines skipped",
			out:  "\naaa=\t1788522801\n\n\nbbb=\t0\n",
			want: map[string]int64{"aaa=": 1788522801, "bbb=": 0},
		},
		{
			name: "no peers configured",
			out:  "",
			want: map[string]int64{},
		},
		{
			name:    "unparsable handshake time",
			out:     "aaa=\tnot-a-number\n",
			wantErr: true,
		},
		{
			name:    "provider failure propagates",
			err:     errors.New("wg: interface not found"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &WgCli{handshakeProvider: fakeHandshakeProvider{out: []byte(tc.out), err: tc.err}}

			peers, err := w.GetPeers()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(peers) != len(tc.want) {
				t.Errorf("got %d peers, want %d", len(peers), len(tc.want))
			}

			for key, ts := range tc.want {
				peer, ok := peers[key]
				if !ok {
					t.Errorf("peer %s missing from result", key)
					continue
				}
				if peer.PublicKey != key {
					t.Errorf("peer %s: PublicKey = %q, want %q", key, peer.PublicKey, key)
				}

				want := time.Unix(ts, 0)
				switch {
				case ts == 0 && peer.HandshakeLastSeen != nil:
					t.Errorf("peer %s: HandshakeLastSeen = %v, want nil", key, *peer.HandshakeLastSeen)
				case ts != 0 && peer.HandshakeLastSeen == nil:
					t.Errorf("peer %s: HandshakeLastSeen = nil, want %v", key, want)
				case ts != 0 && !peer.HandshakeLastSeen.Equal(want):
					t.Errorf("peer %s: HandshakeLastSeen = %v, want %v", key, *peer.HandshakeLastSeen, want)
				}
			}

			for key := range peers {
				if _, ok := tc.want[key]; !ok {
					t.Errorf("unexpected peer %s in result", key)
				}
			}
		})
	}
}

func Test_parseConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				filename: "examples/wg0.conf",
			},
			want: map[string]string{
				"your_public_key_here":   "10.15.1.1:51820",
				"another_public_key":     "10.15.1.2:51820",
				"yet_another_public_key": "",
			},
			wantErr: false,
		},
		{
			name: "more complete example",
			args: args{
				filename: "examples/wg1.conf",
			},
			want: map[string]string{
				"pub_a": "8.8.8.8:5555",
				"pub_b": "1.1.1.1:443",
				"pub_c": "this.is.host:12686",
				"pub_d": "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWireguardConfig(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWireguardConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseWireguardConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWgCli_AttachEndpoints(t *testing.T) {
	type fields struct {
		interfaceName     string
		configFile        string
		handshakeProvider HandshakeData
	}
	type args struct {
		peers map[string]Peer
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]Peer
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				interfaceName:     "wg0",
				configFile:        "examples/wg0.conf",
				handshakeProvider: nil,
			},
			args: args{
				peers: map[string]Peer{
					"your_public_key_here": {
						PublicKey:         "your_public_key_here",
						HandshakeLastSeen: nil,
					},
					"another_public_key": {
						PublicKey:         "your_public_key_here",
						HandshakeLastSeen: nil,
					},
					"yet_another_public_key": {
						PublicKey:         "your_public_key_here",
						HandshakeLastSeen: nil,
					},
				},
			},
			want: map[string]Peer{
				"your_public_key_here": {
					PublicKey:         "your_public_key_here",
					HandshakeLastSeen: nil,
					Endpoint:          "10.15.1.1:51820",
				},
				"another_public_key": {
					PublicKey:         "your_public_key_here",
					HandshakeLastSeen: nil,
					Endpoint:          "10.15.1.2:51820",
				},
				"yet_another_public_key": {
					PublicKey:         "your_public_key_here",
					HandshakeLastSeen: nil,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WgCli{
				interfaceName:     tt.fields.interfaceName,
				configFile:        tt.fields.configFile,
				handshakeProvider: tt.fields.handshakeProvider,
			}
			got, err := w.AttachEndpoints(tt.args.peers)
			if (err != nil) != tt.wantErr {
				t.Errorf("AttachEndpoints() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AttachEndpoints() got = %v, want %v", got, tt.want)
			}
		})
	}
}
