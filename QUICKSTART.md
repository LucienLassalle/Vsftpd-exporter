# Quick Start Guide

## 5 Minute Quick Deployment

### Prerequisites

- Docker and Docker Compose (recommended)
- Or Go 1.24+ (compile from source)

### Method 1: Docker Compose (Easiest) ⭐

```bash
# 1. Clone project
git clone <repository-url>
cd Vsftpd-exporter

# 2. Configure
cp config.example.json config.json
# Edit config.json, fill in your FTP server information

# 3. Start with one command
docker-compose up -d

# 4. Verify service
curl http://localhost:9101/health
```

**Access services**:
- Exporter Metrics: http://localhost:9101/metrics
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

### Method 2: Run binary directly

```bash
# 1. Build
make build

# 2. Configure
cp config.example.json config.json
# Edit configuration file

# 3. Run
./vsftp-exporter -config=./config.json
```

### Method 3: Run from source

```bash
# 1. Install dependencies
go mod download

# 2. Configure
cp config.example.json config.json

# 3. Run
go run vsftp-exporter.go -config=./config.json
```

## Configuration Guide

Minimal configuration example (`config.json`):

```json
{
    "target_host": "192.168.1.100",
    "ftp_port": "21",
    "ftp_user": "ftpuser",
    "ftp_password": "password",
    "need_ssh": false,
    "Xferlog_file_path": "/var/log/xferlog",
    "listen_port": "9101",
    "check_interval": 30
}
```

### SSH Remote Monitoring Configuration

If the FTP server is on a remote host:

```json
{
    "target_host": "192.168.1.100",
    "ftp_port": "21",
    "ftp_user": "ftpuser",
    "ftp_password": "password",
    "need_ssh": true,
    "ssh_port": "22",
    "ssh_user": "root",
    "ssh_password": "ssh_password",
    "Xferlog_file_path": "/var/log/xferlog",
    "listen_port": "9101",
    "check_interval": 30,
    "vsftplog_enabled": true,
    "vsftplog_file_path": "/var/log/vsftpd.log"
}
```

## Verify Deployment

### 1. Check health status

```bash
curl http://localhost:9101/health
```

Expected output:
```json
{
  "status": "healthy",
  "timestamp": "2025-11-17T15:00:00Z",
  "uptime": "5m30s",
  "version": "1.0.0"
}
```

### 2. View metrics

```bash
curl http://localhost:9101/metrics | grep vsftp
```

Should see output like:
```
vsftp_login_success 1
vsftp_connections 5
vsftp_established_connections 3
vsftp_files_sent_total 120
vsftp_files_received_total 85
...
```

### 3. Access Grafana dashboard

1. Open browser: http://localhost:3000
2. Login (default: admin/admin)
3. Navigate to Dashboards → Vsftpd FTP Server Monitoring Dashboard

## Common Issues

### Q: Cannot connect to FTP server

**A**: Check the following:
```bash
# 1. Test FTP connection
telnet <target_host> <ftp_port>

# 2. Check username and password
ftp <target_host>

# 3. View exporter logs
docker logs vsftp-exporter
# or
journalctl -u vsftp-exporter -f
```

### Q: SSH connection failed

**A**: Verify SSH access:
```bash
# Test SSH connection
ssh <ssh_user>@<target_host>

# Check log file permissions
ssh <ssh_user>@<target_host> "ls -l /var/log/xferlog"
```

### Q: Metrics not updating

**A**: Check log file:
```bash
# Confirm log file exists and has new content
tail -f /var/log/xferlog

# Check if exporter is reading
curl http://localhost:9101/metrics | grep vsftp_files
```

### Q: Grafana dashboard shows "No Data"

**A**: Verify data pipeline:
```bash
# 1. Check if Prometheus is scraping data
curl http://localhost:9090/api/v1/targets

# 2. Query Prometheus
curl 'http://localhost:9090/api/v1/query?query=vsftp_login_success'

# 3. Check Grafana data source configuration
# Grafana → Configuration → Data Sources → Prometheus
```

## Next Steps

- 📖 Read [full documentation](README.md)
- 🚀 View [deployment guide](DEPLOYMENT.md)
- 📊 Learn about [Grafana dashboard](GRAFANA_DASHBOARD.md)
- ⚠️ Configure [alert rules](alerts.yml)

## Get Help

- View logs: `docker logs vsftp-exporter`
- Run tests: `make test`
- Validate configuration: `./vsftp-exporter -config=./config.json`

## Production Environment Recommendations

1. **Use Systemd service** - See [DEPLOYMENT.md](DEPLOYMENT.md)
2. **Configure alerts** - Use provided [alerts.yml](alerts.yml)
3. **Regular backups** - Backup Grafana dashboards and Prometheus data
4. **Monitor Exporter** - Configure `VsftpExporterDown` alert
5. **Log rotation** - Ensure FTP log files rotate regularly

---

**Need help?** See [Troubleshooting Guide](README.md#troubleshooting) or submit an Issue.
