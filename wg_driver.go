package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type HandshakeData interface {
	GetHandshakeData() ([]byte, error)
}

type WgCli struct {
	interfaceName     string
	configFile        string
	handshakeProvider HandshakeData
}

func NewWgCli(interfaceName string, configFile string) (*WgCli, error) {
	if len(interfaceName) == 0 {
		return nil, errors.New("empty interface name provided")
	}

	if len(configFile) == 0 {
		return nil, errors.New("empty config file provided")
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		slog.Error("wireguard config file does not exist", "file", configFile)
		os.Exit(1) // Exit with a non-zero status to indicate failure
	}

	return &WgCli{
		interfaceName: interfaceName,
		configFile:    configFile,
		handshakeProvider: &WgHandshakeDataCli{
			interfaceName: interfaceName,
		},
	}, nil
}

func (w *WgCli) StartTunnel() error {
	args := []string{
		"wg-quick",
		"up",
		w.interfaceName,
	}
	cmd := exec.Command(args[0], args[1:]...) //#nosec:G204
	return cmd.Run()
}

func (w *WgCli) IsTunnelUp() (bool, error) {
	cmd := exec.Command("wg", "show")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return false, fmt.Errorf("command failed: %v, output: %s", err, out.String())
	}

	output := out.String()
	if strings.Contains(output, fmt.Sprintf("interface: %s", strings.ToLower(w.interfaceName))) {
		return true, nil
	}

	return false, nil
}

func (w *WgCli) ResetPeer(publicKey string, endpoint string) error {
	args := []string{
		"wg",
		"set",
		w.interfaceName,
		"peer",
		publicKey,
		"endpoint",
		endpoint,
	}
	cmd := exec.Command(args[0], args[1:]...) //#nosec:G204
	return cmd.Run()
}

func (w *WgCli) GetEndpoint(publicKey string) (string, error) {
	config, err := parseWireguardConfig(w.configFile)
	if err != nil {
		return "", err
	}

	for _, peer := range config.Peers {
		if peer.PublicKey == publicKey {
			if peer.Endpoint == nil {
				return "", nil
			}
			return *peer.Endpoint, nil
		}
	}

	return "", fmt.Errorf("public key %s not found", publicKey)
}

func (w *WgCli) GetPeers() ([]Peer, error) {
	output, err := w.handshakeProvider.GetHandshakeData()
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard status: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var peers []Peer

	for _, line := range lines {
		columns := strings.Fields(line)
		if len(columns) < 2 {
			continue
		}

		publicKey := columns[0]
		handshakeTimestamp, err := strconv.ParseInt(columns[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse handshake time: %w", err)
		}

		var handshakeLastSeen *time.Time
		if handshakeTimestamp != 0 {
			hs := time.Unix(handshakeTimestamp, 0)
			handshakeLastSeen = &hs
		}

		peer := Peer{
			PublicKey:         publicKey,
			HandshakeLastSeen: handshakeLastSeen,
		}
		peers = append(peers, peer)
	}

	return peers, nil
}

type WgHandshakeDataCli struct {
	interfaceName string
}

func (w *WgHandshakeDataCli) GetHandshakeData() ([]byte, error) {
	return exec.Command("wg", "show", w.interfaceName, "latest-handshakes").Output() //#nosec:G204
}

func parseWireguardConfig(configFile string) (*WgConfig, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open configuration file: %w", err)
	}
	defer file.Close()

	config := &WgConfig{
		Peers: []Peer{},
	}

	var currentSection string
	var currentPeer Peer

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if currentSection == "Peer" {
				config.Peers = append(config.Peers, currentPeer)
				currentPeer = Peer{} // Reset the peer for the next one
			}
			currentSection = strings.Trim(line, "[]")
			continue
		}

		// Handle key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip malformed lines
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		switch currentSection {
		case "Peer":
			switch key {
			case "PublicKey":
				currentPeer.PublicKey = value
			case "Endpoint":
				currentPeer.Endpoint = &value
			}
		}
	}

	// Add the last peer if any
	if currentSection == "Peer" {
		config.Peers = append(config.Peers, currentPeer)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading configuration file: %w", err)
	}

	return config, nil
}
