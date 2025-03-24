# tunnelguard

[![Go Report Card](https://goreportcard.com/badge/github.com/soerenschneider/tunnelguard)](https://goreportcard.com/report/github.com/soerenschneider/tunnelguard)
![test-workflow](https://github.com/soerenschneider/tunnelguard/actions/workflows/test.yaml/badge.svg)
![release-workflow](https://github.com/soerenschneider/tunnelguard/actions/workflows/release-container.yaml/badge.svg)
![golangci-lint-workflow](https://github.com/soerenschneider/tunnelguard/actions/workflows/golangci-lint.yaml/badge.svg)

**tunnelguard** is a lightweight tool that monitors WireGuard peers for their last handshake and resets the peer if no handshake has been sent within a defined time period. This is especially useful in scenarios where the device on the other end of the WireGuard connection has a dynamic IP address, such as devices using dial-up connections or mobile networks with frequently changing IP addresses.

---

## Features

- **Automatic Peer Reset**: Tunnelguard monitors the handshake times of WireGuard peers and resets the peer if no handshake has occurred within the specified timeout. This helps ensure a clean connection when the remote peer's IP address changes.
- **No Dependencies**: Tunnelguard uses only the Go standard library, making it simple to compile and deploy without worrying about external dependencies.
- **Easy Configuration**: Can be easily configured using a JSON-based configuration file.
- **Metrics Export**: Exports metrics in the Prometheus format for monitoring purposes.

## Installation

To install **tunnelguard**, you can either download the precompiled binaries or compile it from source.

### Precompiled Binaries

You can download the precompiled binary for your platform from the releases section of this repository.

### Building from Source

To build **tunnelguard** from source, you will need [Go](https://golang.org/doc/install) installed. Once Go is installed, follow these steps:

```bash
git clone https://github.com/soerenschneider/tunnelguard.git
cd tunnelguard
make build
```

This will generate a `tunnelguard` binary that you can run directly.

## Configuration

Tunnelguard uses a simple configuration file that defines the WireGuard interface, the configuration file for the WireGuard setup, and the metrics file. Here's the default configuration structure:

## Configuration Options

| Option            | Type   | Default Value                           | Description                                                                         |
|-------------------|--------|-----------------------------------------|-------------------------------------------------------------------------------------|
| wg_interface_name | string | wg0                                     | The name of the WireGuard interface to monitor.                                     |
| wg_config_file    | string | /etc/wireguard/wg0.conf                 | Path to the WireGuard configuration file.                                           |
| pubkey_dict       | dict   |                                         | A mapping of WireGuard public keys to human-readable names for logging and metrics. |
| metrics_file      | string | /var/lib/node_exporter/tunnelguard.prom | File path where Prometheus-compatible metrics are written.                          |

### Example JSON config
```json
{
    "wg_interface_name": "wg0",
    "wg_config_file": "/etc/wireguard/wg0.conf",
    "pubkey_dict": {
      "HUB2HTmOU08ceEe2fQMpzXsBEJoxK+UjV+60rTFZfk8=": "Home Router",
      "4HSO4ReY0T4W6pm9/45KaYSllbHboE+W1s+jnvEZZXw=": "Mobile Device"
    },
    "metrics_file": "/var/lib/node_exporter/tunnelguard.prom"
}
```

## Usage

If the defaults work for you, you will not need to supply a configuration and can just start running it.

```bash
# ./tunnelguard --help
Usage of ./tunnelguard:
  -config string
        Path of config file
  -debug
        Print debug logs
  -version
        Print version and exit

```

## Exported Metrics

Tunnelguard exports Prometheus-compatible metrics for monitoring WireGuard peers. Below is a list of available metrics:

| Metric Name                                            | Type    | Description                                                                                                                                          |
|--------------------------------------------------------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tunnelguard_heartbeat_timestamp_seconds`              | gauge   | The timestamp of the last Tunnelguard invocation.                                                                                                    |
| `tunnelguard_errors_total`                             | counter | Number of errors encountered by Tunnelguard.                                                                                                         |
| `tunnelguard_peers_resets_total`                       | counter | Number of times a WireGuard peer has been reset due to missing handshakes. Includes labels for the peer's public key and its nice name (if defined). |
| `tunnelguard_peers_latest_handshake_timestamp_seconds` | gauge   | The timestamp of a peer's most recent handshake. Includes labels for the peer's public key and its nice name (if defined).                           |
