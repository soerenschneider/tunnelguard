package main

import (
	"errors"
	"reflect"
	"testing"
)

type fakeHandshakeProvider struct {
	out []byte
	err error
}

func (f fakeHandshakeProvider) GetHandshakeData() ([]byte, error) {
	return f.out, f.err
}

func TestGetPeers(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		err       error
		wantKeys  []string
		wantNever []string // subset of wantKeys expected to have nil HandshakeLastSeen
		wantErr   bool
	}{
		{
			name: "three peers, one never handshaked",
			out: "g5MCSkWzApMkKxUr4KXJ6bQpgGcJnEUsrU80658tEww=\t1788522801\n" +
				"+M8GnUBdLQSnCdgdCsX8ufS8H+6pw/IlG+L+0IGGjGc=\t1788522878\n" +
				"Fpmri35N3j6xe44PX7DI/YWNF0mLCL/FF7jWXmeiDl8=\t0\n",
			wantKeys: []string{
				"g5MCSkWzApMkKxUr4KXJ6bQpgGcJnEUsrU80658tEww=",
				"+M8GnUBdLQSnCdgdCsX8ufS8H+6pw/IlG+L+0IGGjGc=",
				"Fpmri35N3j6xe44PX7DI/YWNF0mLCL/FF7jWXmeiDl8=",
			},
			wantNever: []string{"Fpmri35N3j6xe44PX7DI/YWNF0mLCL/FF7jWXmeiDl8="},
		},
		{
			name:     "no trailing newline",
			out:      "aaa=\t1788522801",
			wantKeys: []string{"aaa="},
		},
		{
			name:      "blank lines skipped",
			out:       "\naaa=\t1788522801\n\n\nbbb=\t0\n",
			wantKeys:  []string{"aaa=", "bbb="},
			wantNever: []string{"bbb="},
		},
		{
			name:     "no peers configured",
			out:      "",
			wantKeys: nil,
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
			assertKeys(t, peers, tc.wantKeys)

			never := map[string]bool{}
			for _, k := range tc.wantNever {
				never[k] = true
			}
			for _, p := range peers {
				if got := p.HandshakeLastSeen == nil; got != never[p.PublicKey] {
					t.Errorf("peer %s: HandshakeLastSeen==nil is %v, want %v",
						p.PublicKey, got, never[p.PublicKey])
				}
			}
		})
	}
}

func asPtr(a string) *string {
	return &a
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
