# Grafana Dashboard User Guide

## Dashboard Overview

`grafana-dashboard.json` is a complete Vsftpd FTP server monitoring dashboard configuration file (v2.0), featuring **21 comprehensive monitoring panels** organized into **6 logical rows** with color-coded metrics, advanced visualizations, and real-time monitoring capabilities.

**Dashboard Features:**
- 21 monitoring panels with color-coded thresholds (Green/Yellow/Red)
- 6 hour default time range for better trend visibility
- Auto-refresh every 30 seconds
- Dark theme optimized
- Responsive layout for all screen sizes
- Dynamic datasource and instance selection

## 📊 Panel Layout (21 Panels Total)

### Row 1: 📊 Service Status Overview
Real-time display of FTP service core status metrics (8 stat panels)

1. **FTP Service** - Service status indicator (✅ Online / ❌ Offline) with background color
2. **Total Connections** - Current FTP total connections with color thresholds
3. **Active Connections** - Number of ESTABLISHED connections
4. **Unique Clients** - Number of different client IPs with recent activity
5. **Concurrent Transfers** - Number of currently ongoing file transfers
6. **Active Processes** - Number of currently running vsftpd processes
7. **Total Logins** - Cumulative number of FTP logins with trend graph
8. **Last Login** - Relative time since last successful login (e.g., "5 minutes ago")

### Row 2: 📈 Transfer Statistics
File transfer and bandwidth monitoring with time series graphs

9. **File Transfers Over Time** - Time series showing uploaded and downloaded files
10. **Transfer Rate (Bytes/sec)** - Real-time upload and download transfer rates

### Row 3: 🔌 Connection & Performance Metrics
Connection status and performance analysis

11. **Connection Status Trends** - Time series of total, active, and close-wait connections
12. **Transfer Rate (MB/s)** - Transfer rate visualization in MB/s with legend statistics

### Row 4: 👥 Client Activity & Top Statistics
Client behavior analysis and activity patterns

13. **Top 10 Clients by Connections** - Donut chart with vibrant colors showing client distribution
14. **Client Activity by Hour** - Bar gauge showing connection activity per hour with gradient coloring

### Row 5: ⚠️ Errors & Security Monitoring
Error tracking and security threat detection

15. **Error Metrics** - Time series tracking failed logins, timeouts, and authentication errors
16. **Rapid Reconnections Rate** - Security alert panel for suspicious reconnection patterns with threshold coloring

### Row 6: 📊 Advanced Metrics & Histograms
Advanced performance metrics and distributions

17. **Transfer Duration Distribution** - Histogram showing transfer time distribution
18. **Average Transfer Speed** - Gauge with color thresholds (0-10-50-100 MB/s)
19. **Bandwidth Usage** - Gauge showing current bandwidth with color-coded thresholds
20. **User Login Statistics** - Sortable table displaying top users by login count

## Import Dashboard

### Method 1: Import via Grafana UI

1. Login to Grafana (default: http://localhost:3000)
2. Click left menu "+" → "Import"
3. Click "Upload JSON file"
4. Select `grafana-dashboard.json` file
5. Select Prometheus data source (or use the dynamic datasource variable)
6. Click "Import"

### Method 2: Import via API

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d @grafana-dashboard.json \
  http://localhost:3000/api/dashboards/db
```

### Method 3: Auto-import using Docker Compose

If using the provided `docker-compose.yml`, the dashboard will be automatically configured.

## Dashboard Variables

The dashboard includes three dropdown variables for filtering:

- **Datasource**: Select your Prometheus data source (dynamic selection)
- **Job**: Select monitoring job (default: vsftp-exporter)
- **Instance**: Select monitoring instance (e.g., localhost:9101) - depends on selected datasource and job

## Usage Instructions

### Time Range

- Default displays last **6 hours** of data (extended from 1h for better trend visibility)
- Can adjust via time selector in top right
- Supports auto-refresh (default 30 seconds)

### Panel Interactions

- **Click legend**: Hide/show specific metrics
- **Drag to select**: Zoom into specific time range
- **Double click**: Reset zoom
- **Hover**: View detailed values with min/max/mean/lastNotNull statistics

### Color Coding

All panels use color-coded thresholds:
- **Green**: Normal operation
- **Yellow**: Warning threshold
- **Red**: Critical threshold

## Key Features

### Enhanced Visualizations
- **Stat Panels**: Clean numeric displays with background coloring
- **Time Series**: Full legend tables with min/max/mean statistics
- **Donut Charts**: Vibrant colors for distribution visualization
- **Bar Gauges**: Gradient coloring for activity patterns
- **Histograms**: Transfer duration distribution analysis
- **Gauges**: Color-coded performance indicators
- **Tables**: Sortable user statistics

## Alert Configuration

You can configure alerts for the following panels:

1. **FTP Service** - Service offline alert
2. **Total Connections** - High connection count alert
3. **Active Connections** - Abnormal active connections alert
4. **Error Metrics** - Failed login and timeout alerts
5. **Rapid Reconnections** - Security threat detection

### Alert Configuration Example

1. Click panel title → "Edit"
2. Switch to "Alert" tab
3. Click "Create Alert"
4. Set alert conditions, for example:
   ```
   WHEN last() OF query(A, 5m, now) IS BELOW 1
   ```
5. Configure notification channels
6. Save

**Note**: See `alerts.yml` for pre-configured Prometheus alert rules.

## Customize Dashboard

### Add New Panel

1. Click "Add panel" in top right of dashboard
2. Select visualization type
3. Configure query, for example:
   ```promql
   rate(vsftp_upload_total{job="$job", instance="$instance"}[5m])
   ```
4. Adjust panel settings
5. Save

### Available Prometheus Query Examples

```promql
# Files transferred per minute
rate(vsftp_upload_total[1m]) + rate(vsftp_download_total[1m])

# Transfer error rate
rate(vsftp_transfer_errors_total[5m])

# Average transfer speed (MB/s)
vsftp_average_transfer_speed_bytes_per_second / 1024 / 1024

# Active user count
count(rate(vsftp_user_logins_total[5m]) > 0)

# Client connection distribution (Top 10)
topk(10, rate(vsftp_client_connections_total[5m]))

# Transfer duration P95
histogram_quantile(0.95, rate(vsftp_transfer_duration_seconds_bucket[5m]))

# Bandwidth usage (MB/s)
vsftp_bandwidth_usage_bytes_per_second / 1024 / 1024

# Login failure rate
rate(vsftp_failed_logins_total[5m])

# Client activity by hour
sum by(hour) (increase(vsftp_client_connections_total[1h]))

# Rapid reconnection detection
rate(vsftp_rapid_reconnections_total[5m])
```

## Performance Optimization Recommendations

1. **Adjust refresh interval**: Adjust auto-refresh time as needed (default 30 seconds)
2. **Limit time range**: Use larger time intervals when viewing long time ranges
3. **Use variables**: Use Datasource, Job, and Instance variables to filter data
4. **Panel caching**: Grafana automatically caches query results
5. **Query optimization**: Use rate() and increase() functions for counter metrics

## Troubleshooting

### Dashboard shows "No Data"

1. Check Prometheus data source configuration
2. Verify vsftp-exporter is running normally
3. Confirm Prometheus is scraping metrics (check `/metrics` endpoint)
4. Check if time range is correct (default: last 6 hours)
5. Verify datasource variable is correctly selected

### Query timeout

1. Reduce time range
2. Increase query interval
3. Optimize Prometheus configuration
4. Check Prometheus server resources

### Metrics not updating

1. Check vsftp-exporter logs
2. Verify FTP service has activity
3. Confirm log file path is correct
4. Check auto-refresh is enabled (30s default)

### Colors not showing correctly

1. Verify thresholds are properly configured
2. Check metric values are within expected ranges
3. Refresh browser cache

## Extension Recommendations

You can add the following panels to enhance monitoring:

1. **Statistics by file type** - Show transfer volume for different file extensions
2. **Client geographic distribution** - If GeoIP data is available
3. **Transfer success rate** - Percentage of successful transfers
4. **Detailed error type analysis** - Breakdown of different error categories
5. **User session duration** - Average session length per user
6. **Peak usage times** - Heatmap of busiest hours/days
7. **Storage utilization** - Disk space usage trends
8. **Protocol version distribution** - FTP vs FTPS usage

## Export and Share

### Export Dashboard

1. Click dashboard settings (gear icon)
2. Select "JSON Model"
3. Copy JSON content or download file

### Share Dashboard

1. Click "Share" button
2. Select sharing method:
   - **Link**: Generate share link
   - **Snapshot**: Create snapshot
   - **Export**: Export as JSON

## Version History

- **v2.0** (2025-11-29)
  - Major overhaul with 21 monitoring panels (up from 14)
  - Added 6 logical rows for better organization
  - Implemented color-coded thresholds (Green/Yellow/Red)
  - Extended default time range from 1h to 6h
  - Added dynamic datasource selection variable
  - Enhanced visualizations: donut charts, bar gauges, histograms, gauges, tables
  - Fixed "Last Login" display to show relative time
  - Improved service status panel with color-coded mappings
  - Added new panels: Top 10 Clients, Client Activity by Hour, Error Metrics, Rapid Reconnections, Transfer Duration Distribution, Average Transfer Speed, Bandwidth Usage, User Login Statistics
  - All panels show legend statistics (min/max/mean/lastNotNull)
  - Optimized for dark theme and responsive layouts

- **v1.0** (2025-11-17)
  - Initial version
  - Contains 14 basic monitoring panels
  - Supports service status, transfer statistics, connection monitoring

## Related Documentation

- [README.md](README.md) - Project overview and setup
- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment guide
- [alerts.yml](alerts.yml) - Prometheus alert rules
- [prometheus.yml](prometheus.yml) - Prometheus configuration
- [CHANGELOG.md](CHANGELOG.md) - Version history and changes
