# Vsftpd Exporter Deployment Guide

## Quick Start

### Method 1: Run binary directly

1. **Compile project**
```bash
make build
```

2. **Configuration file**
```bash
cp config.example.json config.json
# Edit config.json to fill in actual configuration
```

3. **Run**
```bash
./vsftp-exporter -config=./config.json
```

### Method 2: Using Docker

1. **Build image**
```bash
docker build -t vsftp-exporter:latest .
```

2. **Run container**
```bash
docker run -d \
  --name vsftp-exporter \
  -p 9101:9101 \
  -v $(pwd)/config.json:/app/config.json:ro \
  vsftp-exporter:latest
```

### Method 3: Using Docker Compose (Recommended)

1. **Prepare configuration file**
```bash
cp config.example.json config.json
# Edit configuration
```

2. **Start all services**
```bash
docker-compose up -d
```

This will start:
- vsftp-exporter (port 9101)
- Prometheus (port 9090)
- Grafana (port 3000)

3. **Access services**
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090
- Metrics: http://localhost:9101/metrics

## Production Environment Deployment

### Systemd Service Configuration

Create `/etc/systemd/system/vsftp-exporter.service`:

```ini
[Unit]
Description=Vsftpd Prometheus Exporter
After=network.target

[Service]
Type=simple
User=prometheus
WorkingDirectory=/opt/vsftp-exporter
ExecStart=/opt/vsftp-exporter/vsftp-exporter -config=/opt/vsftp-exporter/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Start service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable vsftp-exporter
sudo systemctl start vsftp-exporter
sudo systemctl status vsftp-exporter
```

## Monitoring Verification

### Check Exporter Status
```bash
# Health check
curl http://localhost:9101/health

# View metrics
curl http://localhost:9101/metrics
```

### Check Logs
```bash
# Systemd service logs
sudo journalctl -u vsftp-exporter -f

# Docker logs
docker logs -f vsftp-exporter
```

## Troubleshooting

### Common Issues

1. **Unable to connect to FTP server**
   - Check network connection
   - Verify FTP port is correct
   - Confirm username and password are correct

2. **SSH connection failed**
   - Check SSH port and credentials
   - Confirm target server SSH service is running normally
   - Verify network firewall rules

3. **Log file reading failed**
   - Confirm log file path is correct
   - Check file read permissions
   - Verify SSH user has permission to read logs

## Performance Optimization Recommendations

- Adjust `check_interval` based on FTP server load
- Regularly clean up old log files
- Monitor the exporter's own resource usage
