# Vsftpd Exporter for Prometheus

A Prometheus exporter for monitoring vsftpd FTP servers, providing comprehensive FTP service performance and status monitoring metrics.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](Dockerfile)

## 🚀 Quick Start

```bash
# Deploy with Docker Compose in one command
docker-compose up -d

# Access services
# Grafana: http://localhost:3000 (admin/admin)
# Prometheus: http://localhost:9090
# Metrics: http://localhost:9101/metrics
```

📖 For detailed steps, see [Quick Start Guide](QUICKSTART.md)

## Project Overview

Vsftpd Exporter is a Prometheus monitoring exporter specifically designed for vsftpd FTP servers. It collects various monitoring metrics by parsing FTP log files, checking FTP connection status, and performing health checks, helping operations personnel monitor FTP service performance and health in real-time.

### Main Features

- **Connection Monitoring**: Real-time monitoring of FTP connections, concurrent transfers, client connection statistics, etc.
- **Transfer Statistics**: Statistics on file upload/download counts, transferred bytes, transfer speeds, etc.
- **Error Monitoring**: Monitor login failures, transfer errors, connection timeouts and other anomalies
- **Performance Analysis**: Provides performance metrics like transfer duration distribution, bandwidth usage, connection latency, etc.
- **File Statistics**: Statistics on transferred file types by file extension
- **User Activity Monitoring**: Statistics on login and connection activity by username and client IP
- **SSH Remote Monitoring**: Supports connecting to remote servers via SSH to read log files
- **vsftpd Detailed Log Parsing**: Parse vsftpd.log to get more detailed connection and user activity information
- **Health Check**: Periodically check FTP service availability

## Installation and Compilation

### System Requirements

- Go 1.19 or higher
- Running vsftpd FTP server
- Read permission for FTP log files

### Compilation and Installation

```bash
# Clone project
git clone <repository-url>
cd Vsftpd-exporter

# Download dependencies
go mod download

# Compile
go build -o vsftp-exporter vsftp-exporter.go

# Or run directly
go run vsftp-exporter.go
```

### Dependencies

- `github.com/jlaffaye/ftp v0.2.0` - FTP client library
- `github.com/prometheus/client_golang v1.19.1` - Prometheus client library

## Configuration

### Configuration File (config.json)

```json
{
    "target_host": "localhost",       // Target server address
    "ftp_port": "21",                 // FTP server port
    "ftp_user": "testuser",           // FTP username
    "ftp_password": "testpass",       // FTP password
    "tls": false,                     // Enable TLS/FTPS connections
    "skip_tls": false,                // Skip TLS certificate verification (optional)
    "need_ssh": false,                // Whether to connect via SSH to target server
    "ssh_port": "22",                 // SSH connection port
    "ssh_user": "root",               // SSH login username
    "ssh_password": "password",       // SSH login password
    "Xferlog_file_path": "/var/log/xferlog", // FTP transfer log file path
    "listen_port": "9101",            // Exporter listening port
    "check_interval": 30,             // Check interval (seconds)
    "vsftplog_enabled": true,         // Enable vsftpd detailed log parsing
    "vsftplog_file_path": "/var/log/vsftpd.log" // vsftpd detailed log file path
}
```

### Configuration Details

| Configuration | Type | Required | Default | Description |
|--------|------|------|--------|------|
| `target_host` | string | Yes | localhost | Target server address, supports IP address or domain |
| `ftp_port` | string | Yes | 21 | FTP server port number |
| `ftp_user` | string | Yes | - | FTP login username for connection tests |
| `ftp_password` | string | Yes | - | FTP login password for connection tests |
| `tls` | bool | No | false | Enable TLS/FTPS (explicit TLS) for secure FTP connections |
| `skip_tls` | bool | No | false | Skip TLS certificate verification (use with caution, only for self-signed certs) |
| `need_ssh` | bool | No | false | Whether to connect via SSH to target server |
| `ssh_port` | string | No | 22 | SSH connection port |
| `ssh_user` | string | No | - | SSH login username (required when need_ssh is true) |
| `ssh_password` | string | No | - | SSH login password (required when need_ssh is true) |
| `Xferlog_file_path` | string | Yes | /var/log/xferlog | vsftpd transfer log file path |
| `listen_port` | string | No | 9101 | Exporter HTTP service listening port |
| `check_interval` | int | No | 30 | Monitoring check interval (seconds) |
| `vsftplog_enabled` | bool | No | false | Enable vsftpd detailed log parsing |
| `vsftplog_file_path` | string | No | /var/log/vsftpd.log | vsftpd detailed log file path |

## Usage

### Start Exporter

```bash
# Use default configuration file
./vsftp-exporter

# Specify configuration file path
./vsftp-exporter -config=/path/to/config.json
```

### SSH Remote Monitoring Configuration

When you need to monitor vsftpd service on a remote server, you can enable SSH Remote Monitoring feature:

1. **Configure SSH Connection**:
   ```json
   {
       "need_ssh": true,
       "ssh_port": "22",
       "ssh_user": "root",
       "ssh_password": "your_password"
   }
   ```

2. **Ensure SSH Access Permissions**:
   - SSH user needs to have read permission for log files
   - Recommend using key authentication instead of password authentication (production environment)
   - Ensure target server SSH service is running normally

3. **Log File Path**:
   - `Xferlog_file_path`: vsftpdTransfer Log Path（Usually `/var/log/xferlog`）
   - `vsftplog_file_path`: vsftpdDetailed Log Path（Usually `/var/log/vsftpd.log`）

### Verify Running Status

```bash
# Check metrics endpoint
curl http://localhost:9101/metrics

# Check health status
curl http://localhost:9101/health
```

### System Service Configuration

Create systemd service file `/etc/systemd/system/vsftp-exporter.service`:

```ini
[Unit]
Description=Vsftpd Prometheus Exporter
After=network.target

[Service]
Type=simple
User=prometheus
ExecStart=/usr/local/bin/vsftp-exporter
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Start Service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable vsftp-exporter
sudo systemctl start vsftp-exporter
```

## Monitoring Metrics

### Connection Status Metrics

| Metric Name | Type | Description |
|----------|------|------|
| `vsftp_login_success` | Gauge | FTP Login SuccessStatus (1=Success, 0=Failure) |
| `vsftp_connections` | Gauge | Current FTP Total Connections |
| `vsftp_established_connections` | Gauge | EstablishedConnectionCount |
| `vsftp_close_wait_connections` | Gauge | Close WaitConnectionCount |
| `vsftp_concurrent_transfers` | Gauge | CurrentConcurrent Transfers |

### Transfer Statistics Metrics

| Metric Name | Type | Labels | Description |
|----------|------|------|------|
| `vsftp_files_received_total` | Gauge | - | Total Downloaded Files |
| `vsftp_files_sent_total` | Gauge | - | Total Uploaded Files |
| `vsftp_login_total` | Counter | - | FTP Total Login Count |
| `vsftp_upload_total` | Counter | - | FTP total upload operations count |
| `vsftp_download_total` | Counter | - | FTP total download operations count |
| `vsftp_upload_bytes_total` | Counter | - | Total upload bytes |
| `vsftp_download_bytes_total` | Counter | - | Total download bytes |
| `vsftp_transfer_duration_seconds` | Histogram | - | File transfer duration distribution |
| `vsftp_average_transfer_speed_bytes_per_second` | Gauge | - | Average transfer speed (bytes/second) |
| `vsftp_bandwidth_usage_bytes_per_second` | Gauge | - | Current bandwidth usage (bytes/second) |
| `vsftp_last_login_time` | Gauge | - | Timestamp of last successful FTP login |

### Error and Exception Metrics

| Metric Name | Type | Labels | Description |
|----------|------|------|------|
| `vsftp_failed_logins_total` | Counter | - | Total failed login count |
| `vsftp_transfer_errors_total` | Counter | type | Total transfer errors (classified by error type) |
| `vsftp_connection_timeouts_total` | Counter | - | Total connection timeout count |
| `vsftp_authentication_errors_total` | Counter | - | Total authentication errors count |
| `vsftp_max_connections_reached_total` | Counter | - | Count of times max connection limit reached |

### File Statistics Metrics

| Metric Name | Type | Labels | Description |
|----------|------|------|------|
| `vsftp_file_count_by_extension` | Counter | extension | File quantity statistics by file extension |

### Client and User Statistics Metrics

| Metric Name | Type | Labels | Description |
|----------|------|------|------|
| `vsftp_client_connections_total` | Counter | client_ip | Total connections by client IP |
| `vsftp_unique_clients` | Gauge | - | Quantity of unique client IP addresses with recent activity |
| `vsftp_user_logins_total` | Counter | username | Total successful logins by username |
| `vsftp_user_connections_total` | Counter | username | Total connections by username |
| `vsftp_login_failures_by_client` | Counter | client_ip | Login failure count by client IP |
| `vsftp_client_activity_by_hour` | Counter | hour | Client connection activity by hour |
| `vsftp_client_files_total` | Counter | client_ip, direction | Total file transfers by client IP and transfer direction |

### Advanced Monitoring Metrics

| Metric Name | Type | Description |
|----------|------|------|
| `vsftp_connection_login_delay_seconds` | Histogram | Time latency distribution from connection to successful login |
| `vsftp_rapid_reconnections_total` | Counter | Rapid reconnection count (same IP reconnects within 30 seconds) |
| `vsftp_active_processes` | Gauge | Active vsftpd process count based on log entries |

## Prometheus Configuration

Add the following job configuration in Prometheus configuration file:

```yaml
scrape_configs:
  - job_name: 'vsftp-exporter'
    static_configs:
      - targets: ['localhost:9101']
    scrape_interval: 30s
    scrape_timeout: 10s
    metrics_path: /metrics
```

### Alert Rule Examples

```yaml
groups:
  - name: vsftp-alerts
    rules:
      - alert: VsftpdDown
        expr: vsftp_login_success == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Vsftpd service is down"
          description: "Vsftpd service has been down for more than 1 minute"
      
      - alert: HighFailedLogins
        expr: increase(vsftp_failed_logins_total[5m]) > 10
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "High number of failed FTP logins"
          description: "More than 10 failed logins in the last 5 minutes"
      
      - alert: HighTransferErrors
        expr: increase(vsftp_transfer_errors_total[5m]) > 5
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "High number of transfer errors"
          description: "More than 5 transfer errors in the last 5 minutes"
```

## Grafana Dashboard

The project provides a complete Grafana dashboard configuration file `grafana-dashboard.json`, including the following monitoring panels:

### Dashboard Features

- **14+ Monitoring Panels**: Covers service status, transfer statistics, performance analysis, etc.
- **Auto Refresh**: Default 30 seconds auto update data
- **Variable Support**: Supports Job and Instance variable switching
- **Responsive Layout**: Adapts to different screen sizes
- **English Interface**: All panel titles and descriptions are in English

### Panel Groups

**📊 Service Status Overview**
- FTP service status (online/offline)
- Total Connections、Active ConnectionsCount
- Unique Clients、Concurrent Transfers
- Active Processes

**📈 Transfer Statistics**
- Total uploaded/downloaded files
- Total Login Count, Last Login Time
- Connection status trend chart
- Transfer rate chart (MB/s)

### Import Dashboard

**Method 1: Grafana UI Import**

1. Login to Grafana (http://localhost:3000)
2. Click "+" → "Import"
3. Upload grafana-dashboard.json
4. Select Prometheus data source
5. Click "Import"


**Method 2: Using Docker Compose**

```bash
docker-compose up -d
# Dashboard will be automatically configured
```

For detailed usage description, please refer to [GRAFANA_DASHBOARD.md](GRAFANA_DASHBOARD.md)

### Example Query Statements

```promql
# Service availability
vsftp_login_success

# Files transferred per minute
rate(vsftp_upload_total[1m]) + rate(vsftp_download_total[1m])

# Transfer error rate
rate(vsftp_transfer_errors_total[5m]) / (rate(vsftp_upload_bytes_total[5m]) + rate(vsftp_download_bytes_total[5m]))

# Average transfer speed (MB/s)
vsftp_average_transfer_speed_bytes_per_second / 1024 / 1024

# Active user count
count(rate(vsftp_user_logins_total[5m]) > 0)

# Client connection distribution
topk(10, rate(vsftp_client_connections_total[5m]))

# Upload/download ratio
rate(vsftp_upload_bytes_total[5m]) / rate(vsftp_download_bytes_total[5m])

# Total transfer bytes (upload + download)
rate(vsftp_upload_bytes_total[5m]) + rate(vsftp_download_bytes_total[5m])

# Upload traffic (MB/s)
rate(vsftp_upload_bytes_total[5m]) / 1024 / 1024

# Download traffic (MB/s)
rate(vsftp_download_bytes_total[5m]) / 1024 / 1024
```

## Troubleshooting

### Common Issues

**Q: Exporter startup failure, configuration file error tip**

A: Check if config.json file format is correct, ensure all required fields are filled.

**Q: Unable to connect to FTP server**

A: Check the following items:
- FTP server address and port are correct
- Username and password are valid
- Network connection is normal
- Firewall settings allow connection

**Q: Log parsing failure**

A: Confirm：
- Log file path is correct
- Have read permission for log files
- vsftpd log format is standard format

**Q: Metrics data not updating**

A: Check：
- FTP service has activity
- Log file is being updated
- check_interval configuration is reasonable

### Debug Mode

Enable detailed log output:

```bash
./vsftp-exporter -debug
```

### Log Levels

- INFO: Normal running information
- WARN: Warning information
- ERROR: Error information
- DEBUG: Debug information

## Performance Optimization

### Recommended Configuration

- For high-load environments, recommend setting `check_interval` to 15-30 seconds
- Ensure log files are rotated regularly to avoid large files affecting parsing performance
- Monitor the exporter's own resource usage

### Resource Usage

- Memory usage: Usually < 50MB
- CPU usage: Usually < 5%
- Disk I/O: Mainly for reading log files

## Contributing Guidelines

We welcome community contributions! Please follow these steps:

1. Fork this project
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Create a Pull Request

### Development Standards

- Follow Go code standards
- Add appropriate comments and documentation
- Ensure all tests pass
- Update related documentation

### Report Issues

If you find a bug or have a feature recommendation, please submit detailed information in GitHub Issues.

## License

This project is licensed under the MIT License. For details, see the [LICENSE](LICENSE) file.

## Changelog

### v1.0.0
- Initial version release
- Support basic FTP monitoring metrics
- Provide Prometheus integration

---

**Maintainer**: [Your Name]
**Project Homepage**: [Repository URL]
**Issue Feedback**: [Issues URL]
