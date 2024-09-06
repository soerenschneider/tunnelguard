package main

import "testing"

func Test_isDnsEndpoint(t *testing.T) {
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
			name: "ip",
			args: args{
				endpoint: "1.1.1.1:443",
			},
			want:    true,
			wantErr: false,
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
