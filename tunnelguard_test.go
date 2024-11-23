package main

import "testing"

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
