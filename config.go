package main

import (
	"encoding/json"
	"os"
)

const (
	defaultMetricsFile         = "/var/lib/node_exporter/tunnelguard.prom"
	defaultWireguardInterface  = "wg0"
	defaultWireguardConfigFile = "/etc/wireguard/wg0.conf"
)

type TunnelguardConfig struct {
	Interface  string `json:"wg_interface_name"`
	ConfigFile string `json:"wg_config_file"`

	NiceNames map[string]string `json:"nice_names"`

	MetricsFile string `json:"metrics_file"`
}

func getDefault() TunnelguardConfig {
	return TunnelguardConfig{
		Interface:   defaultWireguardInterface,
		ConfigFile:  defaultWireguardConfigFile,
		MetricsFile: defaultMetricsFile,
	}
}

func readConfig(file string) (*TunnelguardConfig, error) {
	conf := getDefault()
	if file == "" {
		return &conf, nil
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &conf)
	return &conf, err
}
