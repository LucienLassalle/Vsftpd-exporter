package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestIsValidHost tests host address validation
func TestIsValidHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"Valid IPv4 address", "192.168.1.1", true},
		{"Valid IPv6 address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"Valid domain", "example.com", true},
		{"Valid subdomain", "ftp.example.com", true},
		{"Invalid empty string", "", false},
		{"Invalid domain (too long)", string(make([]byte, 300)), false},
		{"Invalid special characters", "example@com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidHost(tt.host)
			if result != tt.expected {
				t.Errorf("isValidHost(%q) = %v, expected %v", tt.host, result, tt.expected)
			}
		})
	}
}

// TestIsValidUsername tests username validation
func TestIsValidUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		expected bool
	}{
		{"Valid username (letters)", "testuser", true},
		{"Valid username (alphanumeric)", "user123", true},
		{"Valid username (underscore)", "test_user", true},
		{"Valid username (hyphen)", "test-user", true},
		{"Invalid empty string", "", false},
		{"Invalid special characters", "user@test", false},
		{"Invalid space", "test user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUsername(tt.username)
			if result != tt.expected {
				t.Errorf("isValidUsername(%q) = %v, expected %v", tt.username, result, tt.expected)
			}
		})
	}
}

// TestExtractFileExtension tests file extension extraction
func TestExtractFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"Regular file", "test.txt", "txt"},
		{"File with multiple dots", "archive.tar.gz", "gz"},
		{"No extension", "README", "no_extension"},
		{"Empty string", "", "no_extension"},
		{"Hidden file", ".gitignore", "gitignore"},
		{"Uppercase extension", "FILE.PDF", "pdf"},
		{"File with path", "/path/to/file.log", "log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFileExtension(tt.filename)
			if result != tt.expected {
				t.Errorf("extractFileExtension(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestParseStandardXferlog tests standard xferlog format parsing
func TestParseStandardXferlog(t *testing.T) {
	tests := []struct {
		name              string
		line              string
		expectedDirection string
		expectedClientIP  string
		expectedFileSize  int64
		expectedCompleted bool
	}{
		{
			name:              "Upload completed",
			line:              "Wed Oct 15 16:04:42 2025 1 172.25.235.63 19236361 /txt/yd_platform.txt b _ i g dstore ftp 0 * c",
			expectedDirection: "i",
			expectedClientIP:  "172.25.235.63",
			expectedFileSize:  19236361,
			expectedCompleted: true,
		},
		{
			name:              "Download completed",
			line:              "Wed Oct 15 16:04:42 2025 2 192.168.1.100 1024 /data/file.txt b _ o g testuser ftp 0 * c",
			expectedDirection: "o",
			expectedClientIP:  "192.168.1.100",
			expectedFileSize:  1024,
			expectedCompleted: true,
		},
		{
			name:              "Transfer incomplete",
			line:              "Wed Oct 15 16:04:42 2025 1 172.25.235.63 19236361 /txt/yd_platform.txt b _ i g dstore ftp 0 * i",
			expectedDirection: "i",
			expectedClientIP:  "172.25.235.63",
			expectedFileSize:  19236361,
			expectedCompleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direction, clientIP, fileSize, _, _, _, completed := parseStandardXferlog(tt.line)

			if direction != tt.expectedDirection {
				t.Errorf("direction = %q, expected %q", direction, tt.expectedDirection)
			}
			if clientIP != tt.expectedClientIP {
				t.Errorf("client IP = %q, expected %q", clientIP, tt.expectedClientIP)
			}
			if fileSize != tt.expectedFileSize {
				t.Errorf("file size = %d, expected %d", fileSize, tt.expectedFileSize)
			}
			if completed != tt.expectedCompleted {
				t.Errorf("completion status = %v, expected %v", completed, tt.expectedCompleted)
			}
		})
	}
}

// TestParseVsftpdTimestamp tests vsftpd timestamp parsing
func TestParseVsftpdTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		shouldErr bool
	}{
		{"Valid single digit date", "Wed Oct  6 10:58:33 2025", false},
		{"Valid double digit date", "Wed Oct 16 10:58:33 2025", false},
		{"Valid standard format", "Mon Jan 2 15:04:05 2006", false},
		{"Invalid format", "2025-10-15 16:04:42", true},
		{"Empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVsftpdTimestamp(tt.timeStr)
			if (err != nil) != tt.shouldErr {
				t.Errorf("parseVsftpdTimestamp(%q) error = %v, expected error = %v", tt.timeStr, err, tt.shouldErr)
			}
		})
	}
}

// TestExpandLogFilePath tests log file path expansion
func TestExpandLogFilePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{"Absolute path", "/var/log/xferlog", false},
		{"Relative path", "./log/test.log", false},
		{"Empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandLogFilePath(tt.path)
			if (err != nil) != tt.shouldErr {
				t.Errorf("expandLogFilePath(%q) error = %v, expected error = %v", tt.path, err, tt.shouldErr)
			}
			if !tt.shouldErr && result == "" {
				t.Errorf("expandLogFilePath(%q) returned empty string", tt.path)
			}
		})
	}
}

// TestCheckLogFileAccess tests log file access checking
func TestCheckLogFileAccess(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{"Existing file", testFile, false},
		{"Non-existent file", "/nonexistent/path/file.log", true},
		{"Empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkLogFileAccess(tt.path)
			if (err != nil) != tt.shouldErr {
				t.Errorf("checkLogFileAccess(%q) error = %v, expected error = %v", tt.path, err, tt.shouldErr)
			}
		})
	}
}

// TestExtractTimestamp tests timestamp extraction
func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"Standard format", "2025-10-15 16:04:42 [INFO] Test message"},
		{"Syslog format", "Wed Oct 15 16:04:42 2025 [INFO] Test message"},
		{"No timestamp", "This is a log line without timestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp := extractTimestamp(tt.line)
			if timestamp <= 0 {
				t.Errorf("extractTimestamp(%q) returned invalid timestamp: %d", tt.line, timestamp)
			}
			// Check if timestamp is within reasonable range (should not be 1970 or far future)
			now := time.Now().Unix()
			if timestamp < 946684800 || timestamp > now+86400 { // 2000-01-01 to tomorrow
				t.Errorf("extractTimestamp(%q) returned unreasonable timestamp: %d", tt.line, timestamp)
			}
		})
	}
}

// BenchmarkParseStandardXferlog performance test: parse standard xferlog
func BenchmarkParseStandardXferlog(b *testing.B) {
	line := "Wed Oct 15 16:04:42 2025 1 172.25.235.63 19236361 /txt/yd_platform.txt b _ i g dstore ftp 0 * c"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseStandardXferlog(line)
	}
}

// BenchmarkExtractFileExtension performance test: extract file extension
func BenchmarkExtractFileExtension(b *testing.B) {
	filename := "/path/to/file.txt"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractFileExtension(filename)
	}
}

// BenchmarkIsValidHost performance test: host address validation
func BenchmarkIsValidHost(b *testing.B) {
	host := "ftp.example.com"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isValidHost(host)
	}
}
