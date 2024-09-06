package main

import (
	"reflect"
	"testing"
	"time"
)

var (
	t1 = time.Date(2024, time.September, 5, 17, 45, 18, 0, time.FixedZone("CEST", 2*60*60))
	t2 = time.Date(2024, time.September, 5, 17, 44, 57, 0, time.FixedZone("CEST", 2*60*60))
)

type wgTest struct {
}

func (w *wgTest) GetHandshakeData() ([]byte, error) {
	data := `bbb	1725551118
ccc		1725551097
ddd		0
`
	return []byte(data), nil
}

func TestWg_GetPeers(t *testing.T) {
	type fields struct {
		interfaceName string
		data          HandshakeData
	}
	tests := []struct {
		name    string
		fields  fields
		want    []Peer
		wantErr bool
	}{
		{
			name: "happy",
			fields: fields{
				interfaceName: "wg0",
				data:          &wgTest{},
			},
			want: []Peer{
				{
					PublicKey:         "bbb",
					HandshakeLastSeen: &t1,
				},
				{
					PublicKey:         "ccc",
					HandshakeLastSeen: &t2,
				},
				{
					PublicKey:         "ddd",
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
				handshakeProvider: tt.fields.data,
			}
			got, err := w.GetPeers()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPeers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare slices of Peer structs using custom function
			if len(got) != len(tt.want) {
				t.Errorf("GetPeers() got length = %d, want length %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if !got[i].Equals(&tt.want[i]) {
					t.Errorf("GetPeers() got = %v, want %v", got[i], tt.want[i])
				}
			}
		})
	}
}

// Equals method to compare two Peer structs
func (p *Peer) Equals(other *Peer) bool {
	if p == nil || other == nil {
		return p == other
	}

	if p.PublicKey != other.PublicKey {
		return false
	}

	if !timePtrsEqual(p.HandshakeLastSeen, other.HandshakeLastSeen) {
		return false
	}

	if !stringPtrsEqual(p.Endpoint, other.Endpoint) {
		return false
	}

	return true
}

func asPtr(a string) *string {
	return &a
}

// Helper function to compare two *time.Time pointers
func timePtrsEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

// Helper function to compare two *string pointers
func stringPtrsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func TestWg_GetEndpoint(t *testing.T) {
	type fields struct {
		interfaceName string
		configFile    string
		data          HandshakeData
	}
	type args struct {
		publicKey string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				interfaceName: "wg0",
				configFile:    "examples/wg0.conf",
				data:          nil,
			},
			args: args{
				publicKey: "another_public_key",
			},
			want:    "10.15.1.2:51820",
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				interfaceName: "wg1",
				configFile:    "examples/wg1.conf",
				data:          nil,
			},
			args: args{
				publicKey: "pub_b",
			},
			want:    "1.1.1.1:443",
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				interfaceName: "wg1",
				configFile:    "examples/wg1.conf",
				data:          nil,
			},
			args: args{
				publicKey: "pub_d",
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "unknown pub key",
			fields: fields{
				interfaceName: "wg1",
				configFile:    "examples/wg1.conf",
				data:          nil,
			},
			args: args{
				publicKey: "thisisnotknown",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WgCli{
				interfaceName:     tt.fields.interfaceName,
				configFile:        tt.fields.configFile,
				handshakeProvider: tt.fields.data,
			}
			got, err := w.GetEndpoint(tt.args.publicKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetEndpoint() got = %v, want %v", got, tt.want)
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
		want    *WgConfig
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				filename: "examples/wg0.conf",
			},
			want: &WgConfig{Peers: []Peer{
				{
					PublicKey: "your_public_key_here",
					Endpoint:  asPtr("10.15.1.1:51820"),
				},
				{
					PublicKey: "another_public_key",
					Endpoint:  asPtr("10.15.1.2:51820"),
				},
				{
					PublicKey: "yet_another_public_key",
					Endpoint:  nil,
				},
			}},
			wantErr: false,
		},
		{
			name: "more complete example",
			args: args{
				filename: "examples/wg1.conf",
			},
			want: &WgConfig{Peers: []Peer{
				{
					PublicKey: "pub_a",
					Endpoint:  asPtr("8.8.8.8:5555"),
				},
				{
					PublicKey: "pub_b",
					Endpoint:  asPtr("1.1.1.1:443"),
				},
				{
					PublicKey: "pub_c",
					Endpoint:  asPtr("this.is.host:12686"),
				},
				{
					PublicKey: "pub_d",
					Endpoint:  nil,
				},
			}},
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
