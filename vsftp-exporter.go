// Package main implements a Prometheus exporter for vsftpd FTP server metrics.
// This exporter collects various metrics from vsftpd including:
// - FTP login status and connection counts
// - File transfer statistics (uploads/downloads)
// - Connection state monitoring
// - Log file parsing for historical data
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/ssh"
)

// Logger structured logging component
type Logger struct {
	logger *log.Logger
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
	}
}

// Info logs informational messages
func (l *Logger) Info(msg string, args ...interface{}) {
	l.logger.Printf("[INFO] "+msg, args...)
}

// Warn logs warning messages
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.logger.Printf("[WARN] "+msg, args...)
}

// Error logs error messages
func (l *Logger) Error(msg string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+msg, args...)
}

// Debug logs debug messages
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.logger.Printf("[DEBUG] "+msg, args...)
}

// Global logger instance
var logger = NewLogger()

// Pre-compiled regular expressions for performance
var (
	// vsftpd.log log format regular expressions
	connectRegex = regexp.MustCompile(`^(\w+\s+\w+\s+\d+\s+\d+:\d+:\d+\s+\d+)\s+\[pid\s+(\d+)\]\s+CONNECT:\s+Client\s+"([^"]+)"`)
	loginRegex   = regexp.MustCompile(`^(\w+\s+\w+\s+\d+\s+\d+:\d+:\d+\s+\d+)\s+\[pid\s+(\d+)\]\s+\[([^\]]+)\]\s+OK\s+LOGIN:\s+Client\s+"([^"]+)"`)

	// Host and username validation regular expressions
	domainRegex   = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Config defines the vsftpd exporter configuration structure
// Contains FTP server connection info, monitoring parameters, and log file paths
type Config struct {
	TargetHost       string `json:"target_host"`        // Target server address, supports IP or domain
	FTPPort          string `json:"ftp_port"`           // FTP server port, default 21
	FTPUser          string `json:"ftp_user"`           // FTP login username for connection tests
	FTPPassword      string `json:"ftp_password"`       // FTP login password for connection tests
	TLS              bool   `json:"tls"`                // Enable TLS/SSL for FTP connections
	SkipTLS          bool   `json:"skip_tls,omitempty"` // Skip TLS certificate verification (optional)
	NeedSSH          bool   `json:"need_ssh"`           // Whether to connect via SSH to target server
	SSHPort          string `json:"ssh_port"`           // SSH connection port, default 22
	SSHUser          string `json:"ssh_user"`           // SSH login username
	SSHPassword      string `json:"ssh_password"`       // SSH login password
	LogFilePath      string `json:"Xferlog_file_path"`  // vsftpd log file path for transfer stats
	ListenPort       string `json:"listen_port"`        // Prometheus metrics HTTP service port, default 9100
	CheckInterval    int    `json:"check_interval"`     // Monitoring check interval (seconds), default 30
	VsftplogEnabled  bool   `json:"vsftplog_enabled"`   // Enable vsftpd log parsing
	VsftplogFilePath string `json:"vsftplog_file_path"` // vsftpd detailed log file path
}

// ExporterState maintains the exporter's runtime state
// Tracks log file read positions and handles, supports log rotation detection
type ExporterState struct {
	mu                sync.RWMutex // Mutex to protect concurrent access
	lastProcessedTime time.Time
	ctx               context.Context
	cancel            context.CancelFunc
	logFile           *os.File // Currently opened log file handle
	lastPosition      int64    // Last file read position for incremental reading

	// Fields for tracking transfer statistics
	totalBytesUploaded   int64                // Total uploaded bytes
	totalBytesDownloaded int64                // Total downloaded bytes
	lastBandwidthCheck   time.Time            // Last bandwidth check time
	lastBytesTransferred int64                // Total transferred bytes at last check
	activeTransfers      int                  // Current active transfers count
	transferStartTimes   map[string]time.Time // Transfer start times mapping

	// === State tracking fields based on vsftpd.log ===

	// vsftpd log file related
	vsftpLogFile     *os.File // vsftpd.log file handle
	vsftpLogPosition int64    // vsftpd.log last read position

	// Client and user activity tracking
	clientLastActivity map[string]time.Time // Client IP -> last activity time
	clientConnectTimes map[string]time.Time // Client IP -> last connect time (for login delay)
	userClientMapping  map[string]string    // Username -> Client IP mapping
	activeProcessIDs   map[string]time.Time // Process ID -> last activity time

	// Rapid reconnection detection
	clientLastConnect map[string]time.Time // Client IP -> last connect time (for rapid reconnect detection)

	// Statistics cache for periodic Gauge metric updates
	lastUniqueClientUpdate time.Time // Last unique clients count update time
	lastProcessUpdate      time.Time // Last active processes count update time
}

// Prometheus metric definitions
// These metrics monitor various states and activities of the vsftpd FTP server
var (
	// ftpLoginSuccess indicates FTP server login status
	// Value of 1 indicates recent login test success, 0 indicates failure
	ftpLoginSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_login_success",
		Help: "Indicates if the login to the FTP server is successful (1 for success, 0 for failure).",
	})

	// ftpConnections current total FTP connections
	// Obtained through netstat command statistics
	ftpConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_connections",
		Help: "Current number of FTP connections.",
	})

	// establishedConnections number of FTP connections in ESTABLISHED state
	// Indicates currently active FTP data transfer connections
	establishedConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_established_connections",
		Help: "Number of ESTABLISHED FTP connections.",
	})

	// closeWaitConnections number of FTP connections in CLOSE_WAIT state
	// Indicates connections waiting to close, may indicate connection leak issues
	closeWaitConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_close_wait_connections",
		Help: "Number of CLOSE_WAIT FTP connections.",
	})

	// filesDownloaded total files downloaded from FTP server
	// Cumulative value parsed from log files
	filesDownloaded = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_files_received_total",
		Help: "Total number of files received (downloaded) from the FTP server.",
	})

	// filesUploaded total files uploaded to FTP server
	// Cumulative value parsed from log files
	filesUploaded = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_files_sent_total",
		Help: "Total number of files sent (uploaded) to the FTP server.",
	})

	// ftpLoginTime timestamp of the last successful FTP login
	// Unix timestamp format, used to monitor login activity
	ftpLoginTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_last_login_time",
		Help: "Timestamp of last successful FTP login.",
	})

	// ftpLoginTotal FTP login total count counter
	// Cumulative login count parsed from log files
	ftpLoginTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_login_total",
		Help: "Total number of FTP logins.",
	})

	// ftpUploadTotal FTP upload operations total count counter
	// Cumulative upload count parsed from log files
	ftpUploadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_upload_total",
		Help: "Total number of FTP uploads.",
	})

	// ftpDownloadTotal FTP download operations total count counter
	// Cumulative download count parsed from log files
	ftpDownloadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_download_total",
		Help: "Total number of FTP downloads.",
	})

	// New monitoring metrics

	// uploadBytesTotal total uploaded bytes
	// Statistics of total bytes uploaded
	uploadBytesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_upload_bytes_total",
		Help: "Total number of bytes uploaded.",
	})

	// downloadBytesTotal total downloaded bytes
	// Statistics of total bytes downloaded
	downloadBytesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_download_bytes_total",
		Help: "Total number of bytes downloaded.",
	})

	// transferDurationSeconds file transfer duration distribution (histogram)
	// Records the duration distribution of file transfer operations
	transferDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "vsftp_transfer_duration_seconds",
		Help:    "Duration of file transfers in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // Exponential distribution from 0.1s to 102.4s
	})

	// concurrentTransfers current number of concurrent transfers
	// Real-time statistics of ongoing file transfers
	concurrentTransfers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_concurrent_transfers",
		Help: "Current number of concurrent file transfers.",
	})

	// averageTransferSpeed average transfer speed
	// Calculates average transfer speed in recent period (bytes/second)
	averageTransferSpeed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_average_transfer_speed_bytes_per_second",
		Help: "Average transfer speed in bytes per second.",
	})

	// failedLoginsTotal total failed login attempts
	// Statistics of cumulative FTP login failures
	failedLoginsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_failed_logins_total",
		Help: "Total number of failed login attempts.",
	})

	// transferErrorsTotal total transfer errors (by type)
	// Statistics of different types of transfer errors
	transferErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_transfer_errors_total",
		Help: "Total number of transfer errors by type.",
	}, []string{"type"})

	// connectionTimeoutsTotal total connection timeouts
	// Statistics of cumulative FTP connection timeouts
	connectionTimeoutsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_connection_timeouts_total",
		Help: "Total number of connection timeouts.",
	})

	// authenticationErrorsTotal total authentication errors
	// Statistics of cumulative authentication failures
	authenticationErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_authentication_errors_total",
		Help: "Total number of authentication errors.",
	})

	// maxConnectionsReachedTotal times max connections limit reached
	// Statistics of cumulative times server reached max connections limit
	maxConnectionsReachedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_max_connections_reached_total",
		Help: "Total number of times max connections limit was reached.",
	})

	// bandwidthUsage bandwidth usage rate
	// Real-time monitoring of current bandwidth usage (bytes/second)
	bandwidthUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_bandwidth_usage_bytes_per_second",
		Help: "Current bandwidth usage in bytes per second.",
	})

	// fileCountByExtension file count by extension
	// Statistics of transfer counts by file extension
	fileCountByExtension = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_file_count_by_extension",
		Help: "Number of files transferred by extension.",
	}, []string{"extension"})

	// === New monitoring metrics based on vsftpd.log ===

	// clientConnectionsTotal connections total by client IP
	// Parses CONNECT events from vsftpd.log, categorized by client IP
	clientConnectionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_client_connections_total",
		Help: "Total number of connections by client IP address.",
	}, []string{"client_ip"})

	// uniqueClients current active unique clients count
	// Statistics of different client IPs with activity in recent period
	uniqueClients = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_unique_clients",
		Help: "Number of unique client IP addresses with recent activity.",
	})

	// userLoginsTotal logins total by username
	// Parses OK LOGIN events from vsftpd.log, categorized by username
	userLoginsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_user_logins_total",
		Help: "Total number of successful logins by username.",
	}, []string{"username"})

	// userConnectionsTotal connections total by username
	// Associates CONNECT and LOGIN events, statistics by username
	userConnectionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_user_connections_total",
		Help: "Total number of connections by username.",
	}, []string{"username"})

	// connectionLoginDelaySeconds connection to login delay distribution
	// Statistics of time interval from CONNECT event to OK LOGIN event
	connectionLoginDelaySeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "vsftp_connection_login_delay_seconds",
		Help:    "Time delay between connection and successful login in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // Exponential distribution from 1ms to 16s
	})

	// rapidReconnectionsTotal rapid reconnections count
	// Statistics of repeated connections from same IP within 30 seconds
	rapidReconnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vsftp_rapid_reconnections_total",
		Help: "Total number of rapid reconnections (same IP within 30 seconds).",
	})

	// activeProcesses current active vsftpd processes count
	// Statistics of different process IDs from vsftpd.log
	activeProcesses = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vsftp_active_processes",
		Help: "Number of active vsftpd processes based on log entries.",
	})

	// clientActivityByHour client activity by hour
	// Statistics of client connection activity by time period
	clientActivityByHour = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_client_activity_by_hour",
		Help: "Client connection activity by hour of day.",
	}, []string{"hour"})

	// loginFailuresByClient login failures by client IP
	// Parses login failure events from vsftpd.log
	loginFailuresByClient = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_login_failures_by_client",
		Help: "Number of login failures by client IP address.",
	}, []string{"client_ip"})

	// clientFilesTotal files total by client IP for uploads and downloads
	// Parses file transfer records from xferlog, categorized by client IP and direction
	clientFilesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vsftp_client_files_total",
		Help: "Total number of files transferred by client IP address and direction.",
	}, []string{"client_ip", "direction"})
)

// init initialization function, automatically executed on program start
// Registers all monitoring metrics with Prometheus
func init() {
	// Register all Prometheus metrics to the default registry
	// These metrics will be exposed through the /metrics endpoint
	prometheus.MustRegister(ftpLoginSuccess)        // FTP login status metric
	prometheus.MustRegister(ftpConnections)         // FTP connections count metric
	prometheus.MustRegister(establishedConnections) // Active connections count metric
	prometheus.MustRegister(closeWaitConnections)   // Close-wait connections count metric
	prometheus.MustRegister(filesDownloaded)        // Downloaded files total metric
	prometheus.MustRegister(filesUploaded)          // Uploaded files total metric
	prometheus.MustRegister(ftpLoginTime)           // Last login time metric
	prometheus.MustRegister(ftpLoginTotal)          // Login total count counter
	prometheus.MustRegister(ftpUploadTotal)         // Upload total count counter
	prometheus.MustRegister(ftpDownloadTotal)       // Download total count counter

	// Register new monitoring metrics
	prometheus.MustRegister(uploadBytesTotal)           // Upload bytes total metric
	prometheus.MustRegister(downloadBytesTotal)         // Download bytes total metric
	prometheus.MustRegister(transferDurationSeconds)    // Transfer duration distribution metric
	prometheus.MustRegister(concurrentTransfers)        // Concurrent transfers count metric
	prometheus.MustRegister(averageTransferSpeed)       // Average transfer speed metric
	prometheus.MustRegister(failedLoginsTotal)          // Failed logins total metric
	prometheus.MustRegister(transferErrorsTotal)        // Transfer errors total metric
	prometheus.MustRegister(connectionTimeoutsTotal)    // Connection timeouts total metric
	prometheus.MustRegister(authenticationErrorsTotal)  // Authentication errors total metric
	prometheus.MustRegister(maxConnectionsReachedTotal) // Max connections limit reached metric
	prometheus.MustRegister(bandwidthUsage)             // Bandwidth usage metric
	prometheus.MustRegister(fileCountByExtension)       // File count by extension metric

	// Register new monitoring metrics based on vsftpd.log
	prometheus.MustRegister(clientConnectionsTotal)      // Connections total by client IP metric
	prometheus.MustRegister(uniqueClients)               // Unique clients count metric
	prometheus.MustRegister(userLoginsTotal)             // Logins total by username metric
	prometheus.MustRegister(userConnectionsTotal)        // Connections total by username metric
	prometheus.MustRegister(connectionLoginDelaySeconds) // Connection to login delay distribution metric
	prometheus.MustRegister(rapidReconnectionsTotal)     // Rapid reconnections count metric
	prometheus.MustRegister(activeProcesses)             // Active processes count metric
	prometheus.MustRegister(clientActivityByHour)        // Client activity by hour metric
	prometheus.MustRegister(loginFailuresByClient)       // Login failures by client IP metric
	prometheus.MustRegister(clientFilesTotal)            // Files total by client IP metric
}

// main Program main entry function
// Handles config initialization, monitoring goroutine startup, HTTP server setup and graceful shutdown
func main() {
	// Parse command line arguments
	configFile := flag.String("config", "config.json", "Configuration file path")
	flag.Parse()

	// Step 1: Load and validate configuration file
	// Config file contains FTP server info, monitoring parameters and other key settings
	logger.Info("Loading configuration file: %s", *configFile)
	config, err := loadAndValidateConfig(*configFile)
	if err != nil {
		logger.Error("Configuration loading failed: %v", err)
		os.Exit(1)
	}
	logger.Info("Configuration loaded successfully, target server: %s:%s", config.TargetHost, config.FTPPort)

	// Step 2: Initialize exporter runtime state
	// Maintains log file handles and read positions and other state info
	state := &ExporterState{
		transferStartTimes: make(map[string]time.Time),
		lastBandwidthCheck: time.Now(),
	}

	// Step 3: Create context for graceful shutdown
	// When termination signal received, notify all goroutines to stop via context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Step 4: Setup system signal handling
	// Listen for SIGINT(Ctrl+C) and SIGTERM signals for graceful program shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	logger.Info("Signal handler configured")

	// Step 5: Start background monitoring goroutine
	// Periodically execute monitoring tasks like FTP connection tests, connection count stats and log parsing
	logger.Info("Starting monitoring goroutine, check interval: %d seconds", config.CheckInterval)
	go func() {
		// Create ticker to execute monitoring tasks at configured interval
		ticker := time.NewTicker(time.Duration(config.CheckInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Received stop signal, exit monitoring goroutine
				logger.Info("Monitoring goroutine received stop signal")
				return
			case <-ticker.C:
				// Ticker triggered, execute monitoring tasks

				// Task 1: Check FTP server connection status
				// Try to login to FTP server to verify service availability
				if err := checkFTPLogin(config, state); err != nil {
					logger.Error("FTP connection check failed: %v", err)
					ftpLoginSuccess.Set(0) // Set login failure status
				} else {
					ftpLoginSuccess.Set(1) // Set login success status
				}

				// Task 2: Count current FTP connections
				// Get network connection status via netstat command
				if err := checkConnections(config, state); err != nil {
					logger.Error("Connection check failed: %v", err)
				}

				// Task 3: Parse FTP log file
				// Extract transfer statistics from vsftpd log
				if config.LogFilePath != "" {
					if err := parseFTPLog(config, config.LogFilePath, state); err != nil {
						logger.Error("Failed to parse FTP log: %v", err)
					}
				}

				// Task 4: Parse vsftpd detailed log file
				// Extract connection and login statistics from vsftpd.log
				if config.VsftplogEnabled && config.VsftplogFilePath != "" {
					if err := parseVsftpdLog(config, config.VsftplogFilePath, state); err != nil {
						logger.Error("Failed to parse vsftpd log: %v", err)
					}
				}
			}
		}
	}()

	// Step 6: Configure and start HTTP server
	// Provides Prometheus metrics endpoint and health check endpoint
	server := &http.Server{
		Addr:    ":" + config.ListenPort, // Listen on configured port
		Handler: nil,                     // Use default HTTP multiplexer
	}

	// Register HTTP route handlers
	http.Handle("/metrics", promhttp.Handler())    // Prometheus metrics endpoint
	http.HandleFunc("/health", healthCheckHandler) // Health check endpoint

	// Start HTTP server in separate goroutine to avoid blocking main thread
	go func() {
		logger.Info("Exporter started, listening on port %s", config.ListenPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// If error is not due to normal shutdown, log error and exit
			logger.Error("HTTP server failed to start: %v", err)
			os.Exit(1)
		}
	}()

	// Step 7: Wait for termination signal and execute graceful shutdown
	// Program will block here until SIGINT or SIGTERM signal received
	<-sigChan
	logger.Info("Received shutdown signal, starting graceful shutdown...")

	// Begin graceful shutdown process
	logger.Info("Shutting down server...")
	// Cancel context, notify all goroutines to stop working
	cancel()

	// Clean up resources: close log file handles
	if state.logFile != nil {
		if err := state.logFile.Close(); err != nil {
			logger.Error("Failed to close log file: %v", err)
		} else {
			logger.Info("Log file closed")
		}
	}

	// Gracefully shut down HTTP server
	// Give server 5 seconds to complete currently processing requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown failed: %v", err)
	} else {
		logger.Info("Server shut down gracefully")
	}
}

// loadAndValidateConfig Load and validate configuration file
func loadAndValidateConfig(file string) (*Config, error) {
	var config Config
	configFile, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to open configuration file: %w", err)
	}
	defer configFile.Close()

	byteValue, err := io.ReadAll(configFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read configuration file: %w", err)
	}

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse configuration file: %w", err)
	}

	// Set default values
	if config.FTPPort == "" {
		config.FTPPort = "21"
	}
	if config.ListenPort == "" {
		config.ListenPort = "9100"
	}
	if config.CheckInterval <= 0 {
		config.CheckInterval = 30
	}

	// Validate required configuration items
	if config.TargetHost == "" {
		return nil, fmt.Errorf("Target host address cannot be empty")
	}
	// Validate target host address format
	if !isValidHost(config.TargetHost) {
		return nil, fmt.Errorf("Invalid target host address format: %s", config.TargetHost)
	}

	if config.FTPUser == "" {
		return nil, fmt.Errorf("FTP username cannot be empty")
	}
	// Validate username length and characters
	if len(config.FTPUser) > 64 || !isValidUsername(config.FTPUser) {
		return nil, fmt.Errorf("FTP username format invalid or too long")
	}

	if config.FTPPassword == "" {
		return nil, fmt.Errorf("FTP password cannot be empty")
	}
	// Validate password length
	if len(config.FTPPassword) > 128 {
		return nil, fmt.Errorf("FTP password too long (max 128 characters)")
	}

	// Validate port range
	ftpPort, err := strconv.Atoi(config.FTPPort)
	if err != nil {
		return nil, fmt.Errorf("FTP port number format invalid: %s", config.FTPPort)
	}
	if ftpPort < 1 || ftpPort > 65535 {
		return nil, fmt.Errorf("FTP port must be in range 1-65535")
	}

	listenPort, err := strconv.Atoi(config.ListenPort)
	if err != nil {
		return nil, fmt.Errorf("Listen port number format invalid: %s", config.ListenPort)
	}
	if listenPort < 1 || listenPort > 65535 {
		return nil, fmt.Errorf("Listen port must be in range 1-65535")
	}

	// Validate check interval
	if config.CheckInterval < 1 || config.CheckInterval > 3600 {
		return nil, fmt.Errorf("Check interval must be in range 1-3600 seconds")
	}

	// Validate log file path (if provided)
	if config.LogFilePath != "" {
		// Expand path (support environment variables and relative paths)
		expandedPath, err := expandLogFilePath(config.LogFilePath)
		if err != nil {
			return nil, fmt.Errorf("Log file path processing failed: %w", err)
		}
		// Update config path to expanded absolute path
		config.LogFilePath = expandedPath

		// Use enhanced log file check function
		if err := checkLogFileAccess(config.LogFilePath); err != nil {
			return nil, fmt.Errorf("Log file path validation failed: %w", err)
		}
	} else {
		logger.Warn("Log file path not configured, FTP transfer log parsing will be unavailable")
	}

	return &config, nil
}

// HealthStatus Health check status structure
type HealthStatus struct {
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	Uptime        string    `json:"uptime"`
	LastCheckTime string    `json:"last_check_time,omitempty"`
	Version       string    `json:"version"`
}

var (
	startTime       = time.Now()
	lastHealthCheck time.Time
	appVersion      = "1.0.0" // Application version number
)

// healthCheckHandler Handle health check requests
// Provides detailed HTTP health check endpoint for monitoring system to check service status
// Returns health status information in JSON format
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	lastHealthCheck = time.Now()

	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(startTime).String(),
		Version:   appVersion,
	}

	if !lastHealthCheck.IsZero() {
		status.LastCheckTime = lastHealthCheck.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(status); err != nil {
		logger.Error("Failed to encode health check response: %v", err)
	}
}

// isValidHost Validate host address format (IP address or domain)
func isValidHost(host string) bool {
	// Check if valid IP address
	if net.ParseIP(host) != nil {
		return true
	}
	// Check if valid domain
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	// Use pre-compiled regex to validate domain format
	return domainRegex.MatchString(host)
}

// isValidUsername Validate username format (letters, numbers, underscore, hyphen)
func isValidUsername(username string) bool {
	if len(username) == 0 {
		return false
	}
	// Use pre-compiled regex to validate username format
	return usernameRegex.MatchString(username)
}

// createSSHClient Create SSH client connection
// Establish SSH connection to target server based on config, supports password authentication
// Parameters:
//
//	config: Configuration object containing SSH connection info
//
// Returns:
//
//	*ssh.Client: Returns SSH client on success, nil on failure
//	error: Returns error message on connection failure
func createSSHClient(config *Config) (*ssh.Client, error) {

	// Setup SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: config.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.SSHPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: Production environment should verify host keys
		Timeout:         10 * time.Second,
	}

	// Establish SSH connection
	address := config.TargetHost + ":" + config.SSHPort
	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	// Add INFO log for successful SSH connection
	logger.Info("SSH connection successful: %s@%s:%s", config.SSHUser, config.TargetHost, config.SSHPort)

	return client, nil
}

// executeSSHCommand Execute remote command via SSH
// Execute specified command on target server and return output
// Parameters:
//
//	client: SSH client connection
//	command: Command to execute
//
// Returns:
//
//	string: Command output result
//	error: Returns error message on execution failure
func executeSSHCommand(client *ssh.Client, command string) (string, error) {
	// Create SSH session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("Failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Execute command and get output
	output, err := session.Output(command)
	if err != nil {
		return "", fmt.Errorf("Failed to execute SSH command: %w", err)
	}

	return string(output), nil
}

// checkFTPLogin Check FTP server connection and login status
// Try to connect to configured FTP server and login with provided credentials
// Used to verify FTP service availability and authentication configuration correctness
// Parameters:
//
//	config: Configuration object containing FTP connection info
//	state: Exporter state object for updating related metrics
//
// Returns:
//
//	error: Returns error if connection or login fails, nil on success
func checkFTPLogin(config *Config, state *ExporterState) error {
	// Set connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Establish connection to FTP server
	var conn *ftp.ServerConn
	var err error

	if config.TLS {
		// Use TLS/FTPS connection
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.SkipTLS,
		}
		conn, err = ftp.Dial(config.TargetHost+":"+config.FTPPort, ftp.DialWithExplicitTLS(tlsConfig))
	} else {
		// Use standard FTP connection
		conn, err = ftp.Dial(config.TargetHost + ":" + config.FTPPort)
	}

	if err != nil {
		// Check if timeout error
		if ctx.Err() == context.DeadlineExceeded {
			connectionTimeoutsTotal.Inc()
			return fmt.Errorf("Connection to FTP server timed out: %w", err)
		}
		connectionTimeoutsTotal.Inc()
		return fmt.Errorf("Failed to connect to FTP server: %w", err)
	}
	defer conn.Quit() // Ensure connection is closed when function ends

	// Try to login with configured username and password
	err = conn.Login(config.FTPUser, config.FTPPassword)
	if err != nil {
		// Distinguish authentication errors from other login failures
		if strings.Contains(err.Error(), "530") || strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "login") {
			authenticationErrorsTotal.Inc()
			failedLoginsTotal.Inc()
		} else {
			failedLoginsTotal.Inc()
		}
		return fmt.Errorf("FTP login failed: %w", err)
	}

	return nil // Login successful
}

// checkConnections Check FTP server network connection status
// Decide whether to execute netstat locally or remotely via SSH based on configuration
// Count total connections, established connections and close-wait connections separately
// Parameters:
//
//	config: Configuration object containing FTP port info and SSH config
//	state: Exporter state object (currently unused but reserved for extension)
//
// Returns:
//
//	error: Returns error if netstat command fails, nil on success
func checkConnections(config *Config, state *ExporterState) error {
	var output string

	if config.NeedSSH {
		// Execute netstat command remotely via SSH

		// Create SSH client
		sshClient, err := createSSHClient(config)
		if err != nil {
			return fmt.Errorf("Failed to create SSH connection: %w", err)
		}
		defer sshClient.Close()

		// Execute netstat command remotely
		output, err = executeSSHCommand(sshClient, "netstat -anp")
		if err != nil {
			return fmt.Errorf("Failed to execute netstat command remotely via SSH: %w", err)
		}
	} else {
		// Execute netstat command locally
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "netstat", "-anp")
		outputBytes, err := cmd.Output()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("netstat command execution timed out")
			}
			return fmt.Errorf("Failed to execute netstat command: %w", err)
		}
		output = string(outputBytes)
	}

	// Log complete netstat output (truncate to first 100 lines to avoid overly long logs)
	lines := strings.Split(output, "\n")
	// Process netstat output (removed DEBUG log output)

	// Parse netstat output, count connections
	totalConnections := 0 // Total connections counter
	establishedCount := 0 // Established connections counter
	closeWaitCount := 0   // Close-wait connections counter
	listenCount := 0      // Listening ports counter
	otherStateCount := 0  // Other state connections counter

	// Iterate through each line, find connections containing FTP port
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Check if contains FTP port, supports multiple format matching
		portPattern := ":" + config.FTPPort
		if strings.Contains(line, portPattern) {
			// Check if connection is from vsftpd process
			if strings.Contains(line, "vsftpd") || strings.Contains(line, "ftp") {
				totalConnections++ // Found FTP port connection, increment total count

				// Classify and count by connection state
				if strings.Contains(line, "ESTABLISHED") {
					establishedCount++ // Established connection
				} else if strings.Contains(line, "CLOSE_WAIT") {
					closeWaitCount++ // Connection waiting to close
				} else if strings.Contains(line, "LISTEN") {
					listenCount++ // Listening port
				} else {
					otherStateCount++
				}
			} else {
				// Try to match by port even without process info
				// This is for compatibility with systems where netstat -p may require root permissions
				if strings.Contains(line, "tcp") && strings.Contains(line, portPattern) {
					totalConnections++

					if strings.Contains(line, "ESTABLISHED") {
						establishedCount++
					} else if strings.Contains(line, "CLOSE_WAIT") {
						closeWaitCount++
					} else if strings.Contains(line, "LISTEN") {
						listenCount++
					} else {
						otherStateCount++
					}
				}
			}
		}
	}

	// Update Prometheus metrics
	ftpConnections.Set(float64(totalConnections))         // Set total connections metric
	establishedConnections.Set(float64(establishedCount)) // Set established connections metric
	closeWaitConnections.Set(float64(closeWaitCount))     // Set close-wait connections metric

	return nil // Count complete
}

// extractTimestamp Extract timestamp from log line
// Supports multiple common timestamp formats, converts to Unix timestamp
// Used to track last occurrence time of FTP activity
// Parameters:
//
//	line: Log line text containing timestamp
//
// Returns:
//
//	int64: Returns Unix timestamp if parsed successfully, current time Unix timestamp if failed
func extractTimestamp(line string) int64 {
	// Define multiple timestamp formats and corresponding parse patterns
	timeFormats := []struct {
		regex  *regexp.Regexp
		layout string
	}{
		// Format 1:YYYY-MM-DD HH:MM:SS
		{regexp.MustCompile(`(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`), "2006-01-02 15:04:05"},

		// Format 2:Mon Jan _2 HH:MM:SS YYYY (vsftpd common format, supports single digit dates)
		{regexp.MustCompile(`(\w{3} \w{3}\s+\d{1,2} \d{2}:\d{2}:\d{2} \d{4})`), "Mon Jan _2 15:04:05 2006"},

		// Format 3:Mon Jan 02 15:04:05 2006 (Standard syslog format, double digit dates)
		{regexp.MustCompile(`(\w{3} \w{3} \d{2} \d{2}:\d{2}:\d{2} \d{4})`), "Mon Jan 02 15:04:05 2006"},

		// Format 4:DD/MM/YYYY HH:MM:SS
		{regexp.MustCompile(`(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})`), "02/01/2006 15:04:05"},

		// Format 5:MM/DD/YYYY HH:MM:SS
		{regexp.MustCompile(`(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})`), "01/02/2006 15:04:05"},

		// Format 6:YYYY/MM/DD HH:MM:SS
		{regexp.MustCompile(`(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`), "2006/01/02 15:04:05"},
	}

	// Try parsing various formats, use local timezone
	for _, format := range timeFormats {
		if match := format.regex.FindString(line); match != "" {
			if t, err := time.ParseInLocation(format.layout, match, time.Local); err == nil {
				return t.Unix() // Return Unix timestamp
			}
		}
	}

	// If all formats cannot be parsed, return current time Unix timestamp
	// This avoids displaying incorrect times like "56 years ago"
	return time.Now().Unix()
}

// parseTransferLog Parse transfer log, extract bytes, filename and transfer time
func parseTransferLog(line, direction string) (bytes int64, filename string, duration float64) {
	// Example log format:"OK UPLOAD: Client "192.168.1.100", "/path/to/file.txt", 1024 bytes, 1.5 seconds"
	// Or:"OK DOWNLOAD: Client "192.168.1.100", "/path/to/file.txt", 2048 bytes, 2.3 seconds"

	// Extract bytes
	if bytesMatch := regexp.MustCompile(`(\d+)\s+bytes`).FindStringSubmatch(line); len(bytesMatch) > 1 {
		if b, err := strconv.ParseInt(bytesMatch[1], 10, 64); err == nil {
			bytes = b
		}
	}

	// Extract filename
	if filenameMatch := regexp.MustCompile(`"([^"]+\.[^"]+)"`).FindStringSubmatch(line); len(filenameMatch) > 1 {
		filename = filenameMatch[1]
	}

	// Extract transfer time
	if durationMatch := regexp.MustCompile(`([0-9.]+)\s+seconds`).FindStringSubmatch(line); len(durationMatch) > 1 {
		if d, err := strconv.ParseFloat(durationMatch[1], 64); err == nil {
			duration = d
		}
	}

	// If specific info not found, use default values
	if bytes == 0 {
		bytes = 1024 // Default 1KB
	}
	if filename == "" {
		filename = "unknown.txt"
	}
	if duration == 0 {
		duration = 1.0 // Default 1 second
	}

	return bytes, filename, duration
}

// extractFileExtension Extract extension from filename
func extractFileExtension(filename string) string {
	if filename == "" {
		return "no_extension"
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filename))

	// If no extension, return"no_extension"
	if ext == "" {
		return "no_extension"
	}

	// Return all extensions (remove dot)
	return ext[1:] // Remove leading dot
}

// expandLogFilePath Expand log file path, support environment variables and relative paths
// Parameters:
//
//	path: Original path, may contain environment variables or relative paths
//
// Returns:
//
//	string: Expanded absolute path
//	error: Returns error if path processing fails
func expandLogFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("Log file path cannot be empty")
	}

	// Expand environment variables
	expandedPath := os.ExpandEnv(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(expandedPath)
	if err != nil {
		return "", fmt.Errorf("Unable to convert to absolute path: %w", err)
	}

	// Clean path (remove redundant separators etc)
	cleanPath := filepath.Clean(absPath)

	return cleanPath, nil
}

// testLogFileAccess Test log file accessibility
// This is a standalone test function that can be called at program start or as needed
// Parameters:
//
//	logPath: Log file path
//
// Returns:
//
//	bool: Whether file is accessible
//	string: Detailed test result information
func testLogFileAccess(logPath string) (bool, string) {
	var results []string

	// Test path expansion
	expandedPath, err := expandLogFilePath(logPath)
	if err != nil {
		return false, fmt.Sprintf("Path expansion failed: %v", err)
	}
	results = append(results, fmt.Sprintf("✓ Path expansion successful: %s -> %s", logPath, expandedPath))

	// Test file accessibility
	err = checkLogFileAccess(expandedPath)
	if err != nil {
		return false, fmt.Sprintf("Accessibility check failed: %v\nCompleted checks:\n%s", err, strings.Join(results, "\n"))
	}
	results = append(results, "✓ File accessibility check passed")

	// Test file reading (read first few lines)
	file, err := os.Open(expandedPath)
	if err != nil {
		return false, fmt.Sprintf("File open failed: %v\nCompleted checks:\n%s", err, strings.Join(results, "\n"))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() && lineCount < 3 {
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Sprintf("File read failed: %v\nCompleted checks:\n%s", err, strings.Join(results, "\n"))
	}
	results = append(results, fmt.Sprintf("✓ File read test passed, read %d lines", lineCount))

	return true, fmt.Sprintf("All tests passed:\n%s", strings.Join(results, "\n"))
}

// checkLogFileAccess Check log file existence, permissions and readability
// Provides detailed error messages to help diagnose issues
// Parameters:
//
//	logPath: Log file path
//
// Returns:
//
//	error: Returns detailed error message if check fails, nil on success
func checkLogFileAccess(logPath string) error {
	// Check if path is empty
	if logPath == "" {
		return fmt.Errorf("Log file path is empty")
	}

	// Check if file exists
	fileInfo, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		// Check if parent directory exists
		dir := filepath.Dir(logPath)
		if _, dirErr := os.Stat(dir); os.IsNotExist(dirErr) {
			return fmt.Errorf("Log file does not exist and parent directory does not exist: %s (Parent directory: %s)", logPath, dir)
		}
		return fmt.Errorf("Log file does not exist: %s (Please check xferlog_file setting in vsftpd configuration)", logPath)
	}
	if err != nil {
		return fmt.Errorf("Cannot access log file: %s, error: %v", logPath, err)
	}

	// Check if regular file
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("Specified path is not a regular file: %s (File type: %s)", logPath, fileInfo.Mode().String())
	}

	// Check if file is readable
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("No permission to read log file: %s (Current user may need read permission)", logPath)
		}
		return fmt.Errorf("Cannot open log file: %s, error: %v", logPath, err)
	}
	file.Close()

	// Check file size (optional warning)
	if fileInfo.Size() == 0 {
		logger.Warn("Log file is empty: %s (this may be normal if vsftpd just started)", logPath)
	}

	return nil
}

// readRemoteFile reads remote file content via SSH
// Supports incremental reading, only reads new content from specified position
func readRemoteFile(config *Config, filePath string, startPosition int64) ([]string, int64, error) {
	if !config.NeedSSH {
		// If SSH not needed, read local file directly
		return readLocalFile(filePath, startPosition)
	}

	// Add INFO log for SSH file read start
	logger.Info("Connecting via SSH to %s reading file: %s", config.TargetHost, filePath)

	// Create SSH connection
	sshClient, err := createSSHClient(config)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to create SSH connection: %w", err)
	}
	defer sshClient.Close()

	// Use tail command to read file from specified position
	var command string
	if startPosition > 0 {
		// Use dd command to skip already read bytes
		command = fmt.Sprintf("dd if=%s bs=1 skip=%d 2>/dev/null", filePath, startPosition)
	} else {
		// Read entire file
		command = fmt.Sprintf("cat %s", filePath)
	}

	output, err := executeSSHCommand(sshClient, command)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to execute SSH command: %w", err)
	}

	lines := strings.Split(output, "\n")
	// Remove last empty line
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Calculate new position
	newPosition := startPosition + int64(len(output))

	// Add INFO log for successful SSH file read
	logger.Info("SSH file read successful, read %d lines, new position: %d", len(lines), newPosition)

	return lines, newPosition, nil
}

// readLocalFile Read local file content (for cases not requiring SSH)
func readLocalFile(filePath string, startPosition int64) ([]string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to open local file: %w", err)
	}
	defer file.Close()

	// Seek to specified position
	if _, err := file.Seek(startPosition, 0); err != nil {
		return nil, 0, fmt.Errorf("Failed to seek file position: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("Failed to read file: %w", err)
	}

	// Get current file position
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to get file position: %w", err)
	}

	return lines, currentPos, nil
}

// parseStandardXferlog Parse standard xferlog format
// Standard format:Wed Oct 15 16:04:42 2025 1 172.25.235.63 19236361 /txt/yd_platform.txt b _ o g dstore ftp 0 * c
// Field description: timestamp transfer_time(seconds) client_ip file_size(bytes) file_path transfer_type special_action_flag direction access_mode username service_name auth_method auth_user_id completion_status
func parseStandardXferlog(line string) (direction string, clientIP string, fileSize int64, filePath string, transferTime int, username string, completed bool) {
	fields := strings.Fields(line)
	if len(fields) < 18 {
		return "", "", 0, "", 0, "", false
	}

	// Parse fields (based on standard xferlog format)
	// Format: Wed Oct 15 16:04:42 2025 1 172.25.235.63 19236361 /txt/yd_platform.txt b _ o g dstore ftp 0 * c
	transferTimeStr := fields[5]   // Transfer time (seconds)
	clientIP = fields[6]           // client_ip
	fileSizeStr := fields[7]       // File size (bytes)
	filePath = fields[8]           // file_path
	direction = fields[11]         // direction: o (outbound/download) or i (inbound/upload)
	username = fields[13]          // username
	completionStatus := fields[17] // completion_status: c (complete) or i (incomplete)

	// Parse transfer time
	if t, err := strconv.Atoi(transferTimeStr); err == nil {
		transferTime = t
	}

	// Parse file size
	if size, err := strconv.ParseInt(fileSizeStr, 10, 64); err == nil {
		fileSize = size
	}

	// Check if completed
	completed = (completionStatus == "c")

	return direction, clientIP, fileSize, filePath, transferTime, username, completed
}

// parseFTPLog Parse FTP log file and update related metrics
// Supports standard xferlog format and SSH remote reading
// Uses incremental reading, only processes new log content since last read
// Parameters:
//
//	config: Configuration object containing SSH connection info
//	logPath: Full FTP log file path
//	state: Exporter state object for maintaining read positions
//
// Returns:
//
//	error: Returns error if file operation or parsing fails, nil on success
func parseFTPLog(config *Config, logPath string, state *ExporterState) error {
	// Add INFO log for starting parsing
	logger.Info("Starting to parse FTP log file: %s, starting from position %d", logPath, state.lastPosition)

	// Read log file content
	lines, newPosition, err := readRemoteFile(config, logPath, state.lastPosition)
	if err != nil {
		return fmt.Errorf("Failed to read log file: %w", err)
	}

	linesProcessed := 0
	loginCount := 0
	uploadCount := 0
	downloadCount := 0
	const maxLinesPerRead = 1000 // Limit number of lines processed per iteration

	// Temporary variables for bandwidth calculation
	currentTime := time.Now()
	totalBytesThisRound := int64(0)

	for _, line := range lines {
		if linesProcessed >= maxLinesPerRead {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		linesProcessed++

		// Try parsing standard xferlog format
		direction, clientIP, fileSize, filePath, transferTime, username, completed := parseStandardXferlog(line)

		if direction != "" && completed {
			// Update client connection statistics
			if clientIP != "" {
				clientConnectionsTotal.WithLabelValues(clientIP).Inc()
			}

			// Update user statistics
			if username != "" {
				userConnectionsTotal.WithLabelValues(username).Inc()
			}

			// Count uploads/downloads by direction
			if direction == "i" { // Inbound = Upload to server
				uploadCount++
				ftpUploadTotal.Inc()
				filesUploaded.Inc() // Update Gauge metric

				// Count uploaded files by client_ip
				if clientIP != "" {
					clientFilesTotal.WithLabelValues(clientIP, "upload").Inc()
				}

				if fileSize > 0 {
					uploadBytesTotal.Add(float64(fileSize))
					state.totalBytesUploaded += fileSize
					totalBytesThisRound += fileSize
				}

				// Record transfer time
				if transferTime > 0 {
					transferDurationSeconds.Observe(float64(transferTime))
				}

				// Count file extension
				if ext := extractFileExtension(filePath); ext != "" {
					fileCountByExtension.WithLabelValues(ext).Inc()
				}

			} else if direction == "o" { // Outbound = Download from server
				downloadCount++
				ftpDownloadTotal.Inc()
				filesDownloaded.Inc() // Update Gauge metric

				// Count downloaded files by client_ip
				if clientIP != "" {
					clientFilesTotal.WithLabelValues(clientIP, "download").Inc()
				}

				if fileSize > 0 {
					downloadBytesTotal.Add(float64(fileSize))
					state.totalBytesDownloaded += fileSize
					totalBytesThisRound += fileSize
				}

				// Record transfer time
				if transferTime > 0 {
					transferDurationSeconds.Observe(float64(transferTime))
				}

				// Count file extension
				if ext := extractFileExtension(filePath); ext != "" {
					fileCountByExtension.WithLabelValues(ext).Inc()
				}
			}
		}

		// Parse old format for backward compatibility
		// Parse successful login log
		if strings.Contains(line, "OK LOGIN") {
			loginCount++
			// Try parsing timestamp
			if timestamp := extractTimestamp(line); timestamp > 0 {
				ftpLoginTime.Set(float64(timestamp))
			} else {
				// If unable to parse timestamp, use current time
				// This avoids displaying incorrect times like "56 years ago"
				ftpLoginTime.Set(float64(time.Now().Unix()))
			}
			ftpLoginTotal.Inc()
		}

		// Parse failed login log
		if strings.Contains(line, "FAIL LOGIN") || strings.Contains(line, "530") {
			failedLoginsTotal.Inc()
			if strings.Contains(line, "530") {
				authenticationErrorsTotal.Inc()
			}
		}

		// Parse transfer errors
		if strings.Contains(line, "FAIL UPLOAD") {
			transferErrorsTotal.WithLabelValues("upload").Inc()
		} else if strings.Contains(line, "FAIL DOWNLOAD") {
			transferErrorsTotal.WithLabelValues("download").Inc()
		} else if strings.Contains(line, "timeout") || strings.Contains(line, "TIMEOUT") {
			transferErrorsTotal.WithLabelValues("timeout").Inc()
			connectionTimeoutsTotal.Inc()
		}

		// Parse max connections limit
		if strings.Contains(line, "max connections") || strings.Contains(line, "connection limit") {
			maxConnectionsReachedTotal.Inc()
		}
	}

	// Update read position
	state.lastPosition = newPosition

	// Update concurrent transfers count
	concurrentTransfers.Set(float64(state.activeTransfers))

	// Calculate and update bandwidth usage
	if !state.lastBandwidthCheck.IsZero() {
		timeDiff := currentTime.Sub(state.lastBandwidthCheck).Seconds()
		if timeDiff > 0 {
			// Calculate bandwidth rate for current round
			currentBandwidthRate := float64(totalBytesThisRound) / timeDiff
			bandwidthUsage.Set(currentBandwidthRate)

			// Calculate cumulative average transfer speed
			totalBytes := state.totalBytesUploaded + state.totalBytesDownloaded
			if totalBytes > 0 {
				// Calculate average speed using program runtime
				programRunTime := currentTime.Sub(state.lastProcessedTime).Seconds()
				if programRunTime > 0 {
					averageSpeed := float64(totalBytes) / programRunTime
					averageTransferSpeed.Set(averageSpeed)
				}
			}
		}
	} else {
		// Initialize on first run
		state.lastBandwidthCheck = currentTime
	}

	// Update check time and cumulative bytes
	state.lastBandwidthCheck = currentTime
	state.lastBytesTransferred += totalBytesThisRound

	// Add INFO log for parsing completion
	logger.Info("FTP log parsing complete, processed %d lines, uploads: %d, downloads: %d", linesProcessed, uploadCount, downloadCount)

	return nil
}

// parseVsftpdLog Parse vsftpd.log file, extract connection and login event information
// Supports parsing CONNECT and OK LOGIN events, updates related monitoring metrics
// Supports SSH remote reading
func parseVsftpdLog(config *Config, logPath string, state *ExporterState) error {
	if logPath == "" {
		return nil // If vsftpd.log path not configured, return directly
	}

	// Add INFO log for starting parsing
	logger.Info("Starting to parse vsftpd log file: %s", logPath)

	// Initialize state maps (if not already initialized)
	if state.clientLastActivity == nil {
		state.clientLastActivity = make(map[string]time.Time)
	}
	if state.clientConnectTimes == nil {
		state.clientConnectTimes = make(map[string]time.Time)
	}
	if state.userClientMapping == nil {
		state.userClientMapping = make(map[string]string)
	}
	if state.activeProcessIDs == nil {
		state.activeProcessIDs = make(map[string]time.Time)
	}
	if state.clientLastConnect == nil {
		state.clientLastConnect = make(map[string]time.Time)
	}

	// Read vsftpd log file content
	lines, newPosition, err := readRemoteFile(config, logPath, state.vsftpLogPosition)
	if err != nil {
		return fmt.Errorf("Failed to read vsftpd log file: %w", err)
	}

	// Update read position
	state.vsftpLogPosition = newPosition

	linesProcessed := 0
	connectCount := 0
	loginCount := 0
	currentTime := time.Now()

	// Use pre-compiled global regex to parse vsftpd.log format
	// Example: Wed Oct 15 15:34:29 2025 [pid 2] CONNECT: Client "172.25.235.63"
	// Example: Wed Oct 15 15:34:29 2025 [pid 1] [ostore] OK LOGIN: Client "172.25.235.63"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		linesProcessed++

		// Parse CONNECT event
		if matches := connectRegex.FindStringSubmatch(line); matches != nil {
			timeStr := matches[1]
			processID := matches[2]
			clientIP := matches[3]

			// Parse timestamp
			eventTime, err := parseVsftpdTimestamp(timeStr)
			if err != nil {
				logger.Warn("Failed to parse timestamp: %s, error: %v", timeStr, err)
				continue
			}

			// Update metrics
			clientConnectionsTotal.WithLabelValues(clientIP).Inc()
			connectCount++

			// Update state tracking
			state.clientLastActivity[clientIP] = eventTime
			state.clientConnectTimes[clientIP] = eventTime
			state.activeProcessIDs[processID] = eventTime

			// Detect rapid reconnections (same IP reconnects within 30 seconds)
			if lastConnect, exists := state.clientLastConnect[clientIP]; exists {
				if eventTime.Sub(lastConnect).Seconds() <= 30 {
					rapidReconnectionsTotal.Inc()
				}
			}
			state.clientLastConnect[clientIP] = eventTime

			// Count client activity by hour
			hour := fmt.Sprintf("%02d", eventTime.Hour())
			clientActivityByHour.WithLabelValues(hour).Inc()
		}

		// Parse OK LOGIN event
		if matches := loginRegex.FindStringSubmatch(line); matches != nil {
			timeStr := matches[1]
			processID := matches[2]
			username := matches[3]
			clientIP := matches[4]

			// Parse timestamp
			eventTime, err := parseVsftpdTimestamp(timeStr)
			if err != nil {
				logger.Warn("Failed to parse timestamp: %s, error: %v", timeStr, err)
				continue
			}

			// Update metrics
			userLoginsTotal.WithLabelValues(username).Inc()
			userConnectionsTotal.WithLabelValues(username).Inc()
			ftpLoginTotal.Inc() // Fix: add total login count
			loginCount++

			// Update last login time metric
			ftpLoginTime.Set(float64(eventTime.Unix()))

			// Update state tracking
			state.clientLastActivity[clientIP] = eventTime
			state.userClientMapping[username] = clientIP
			state.activeProcessIDs[processID] = eventTime

			// Calculate connection to login delay
			if connectTime, exists := state.clientConnectTimes[clientIP]; exists {
				delay := eventTime.Sub(connectTime).Seconds()
				if delay >= 0 && delay <= 60 { // Reasonable delay range (0-60 seconds)
					connectionLoginDelaySeconds.Observe(delay)
				}
			}
		}
	}

	// Periodically update Gauge metrics (every minute)
	if currentTime.Sub(state.lastUniqueClientUpdate).Minutes() >= 1 {
		updateUniqueClientsMetric(state, currentTime)
		state.lastUniqueClientUpdate = currentTime
	}

	if currentTime.Sub(state.lastProcessUpdate).Minutes() >= 1 {
		updateActiveProcessesMetric(state, currentTime)
		state.lastProcessUpdate = currentTime
	}

	// Add INFO log for parsing completion
	logger.Info("vsftpd log parsing complete, processed %d lines, connections: %d, logins: %d", linesProcessed, connectCount, loginCount)

	return nil
}

// parseVsftpdTimestamp Parse timestamp format in vsftpd.log
// Format: Wed Oct 15 15:34:29 2025
func parseVsftpdTimestamp(timeStr string) (time.Time, error) {
	// vsftpd time format, supports single-digit dates (with leading space)
	layouts := []string{
		"Mon Jan _2 15:04:05 2006", // Supports single-digit dates, e.g. "Thu Oct  6 10:58:33 2025"
		"Mon Jan 02 15:04:05 2006", // Supports double-digit dates, e.g. "Thu Oct 16 10:58:33 2025"
		"Mon Jan 2 15:04:05 2006",  // Standard format
	}

	// Parse time using local timezone instead of UTC
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, timeStr, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("Unable to parse timestamp: %s", timeStr)
}

// updateUniqueClientsMetric Update unique clients count metric
// Count different client_ip addresses with activity in last 5 minutes
func updateUniqueClientsMetric(state *ExporterState, currentTime time.Time) {
	activeClients := 0
	cutoffTime := currentTime.Add(-5 * time.Minute) // Activity in last 5 minutes

	for clientIP, lastActivity := range state.clientLastActivity {
		if lastActivity.After(cutoffTime) {
			activeClients++
		} else {
			// Clean up expired client records
			delete(state.clientLastActivity, clientIP)
			delete(state.clientConnectTimes, clientIP)
		}
	}

	uniqueClients.Set(float64(activeClients))
}

// updateActiveProcessesMetric Update active processes count metric
// Count different process IDs with activity in last 5 minutes
func updateActiveProcessesMetric(state *ExporterState, currentTime time.Time) {
	activeProcessCount := 0
	cutoffTime := currentTime.Add(-5 * time.Minute) // Activity in last 5 minutes

	for processID, lastActivity := range state.activeProcessIDs {
		if lastActivity.After(cutoffTime) {
			activeProcessCount++
		} else {
			// Clean up expired process records
			delete(state.activeProcessIDs, processID)
		}
	}

	activeProcesses.Set(float64(activeProcessCount))
}
