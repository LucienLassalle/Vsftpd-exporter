# Project Structure Documentation

## 📁 Directory Structure

```
Vsftpd-exporter/
├── 📄 Core Code
│   ├── vsftp-exporter.go          # Main program (1668 lines)
│   └── vsftp-exporter_test.go     # Unit tests (275 lines)
│
├── ⚙️ Configuration Files
│   ├── config.json                 # Actual configuration (not committed to Git)
│   ├── config.example.json         # Configuration template
│   ├── prometheus.yml              # Prometheus configuration example
│   └── alerts.yml                  # Alert rules (15 rules)
│
├── 🐳 Containerization
│   ├── Dockerfile                  # Docker image build
│   └── docker-compose.yml          # Complete monitoring stack deployment
│
├── 📊 Monitoring & Visualization
│   ├── grafana-dashboard.json      # Grafana dashboard (21 panels)
│   └── GRAFANA_DASHBOARD.md        # Dashboard user guide
│
├── 📚 Documentation
│   ├── README.md                   # Project overview and usage
│   ├── QUICKSTART.md               # 5-minute quick start
│   ├── DEPLOYMENT.md               # Deployment guide
│   ├── AGENTS.md                   # Development guidelines
│   ├── CHANGELOG.md                # Change log
│   ├── OPTIMIZATION_SUMMARY.md     # Optimization summary report
│   └── PROJECT_STRUCTURE.md        # This file
│
├── 🔧 Build & Tools
│   ├── Makefile                    # Build script
│   ├── go.mod                      # Go module definition
│   ├── go.sum                      # Dependency checksums
│   ├── validate_dashboard.py       # Dashboard validation tool
│   └── final_check.sh              # Final validation script
│
├── 🤖 CI/CD
│   └── .github/workflows/
│       ├── ci.yml                  # Continuous integration
│       └── release.yml             # Automated release
│
├── 📝 Other
│   ├── .gitignore                  # Git ignore rules
│   ├── LICENSE                     # MIT License
│   └── log/                        # Log sample directory
│
└── 🔨 Build Artifacts (not committed)
    └── vsftp-exporter              # Compiled binary
```

## 📄 File Descriptions

### Core Code

#### vsftp-exporter.go
Main program file containing:
- FTP connection monitoring
- Log parsing (xferlog and vsftpd.log)
- SSH remote support
- Prometheus metrics export
- 40+ monitoring metrics

#### vsftp-exporter_test.go
Unit test file containing:
- 8 test functions
- 38 test cases
- 3 performance benchmark tests
- Race condition detection support

### Configuration Files

#### config.json
Actual configuration file (contains sensitive information, not committed to Git)

#### config.example.json
Configuration file template containing:
- All configuration options with descriptions
- Default value examples
- Comments explaining each option

#### prometheus.yml
Prometheus configuration example containing:
- Scrape configuration
- Target definitions
- Label settings

#### alerts.yml
Alert rule configuration with 15 rules:
- Service status alerts
- Performance alerts
- Error alerts
- Security alerts

### Containerization

#### Dockerfile
Multi-stage build configuration:
- Build stage: Compile Go program
- Runtime stage: Alpine Linux + necessary tools
- Non-root user execution
- Health check configuration

#### docker-compose.yml
Complete monitoring stack containing:
- vsftp-exporter
- Prometheus
- Grafana
- Network and volume configuration

### Monitoring & Visualization

#### grafana-dashboard.json
Complete Grafana dashboard:
- 21 monitoring panels (updated from 14)
- 6 panel groups (updated from 2)
- Variable support (Datasource, Job, Instance)
- Auto-refresh (30 seconds)
- Extended time range (6 hours default)

#### GRAFANA_DASHBOARD.md
Dashboard user guide:
- Import methods
- Panel descriptions
- Query examples
- Customization recommendations

### Documentation

#### README.md
Main project documentation containing:
- Project introduction
- Installation instructions
- Configuration guide
- Usage methods
- Troubleshooting

#### QUICKSTART.md
Quick start guide:
- 5-minute deployment
- 3 deployment methods
- Common issues
- Verification steps

#### DEPLOYMENT.md
Detailed deployment guide:
- Multiple deployment methods
- Production environment configuration
- Systemd service
- Monitoring verification

#### AGENTS.md
Development guidelines:
- Code style
- Commit conventions
- Testing requirements
- Security recommendations

#### CHANGELOG.md
Change log:
- Version history
- New features
- Optimization improvements
- Pending tasks

#### OPTIMIZATION_SUMMARY.md
Optimization summary report:
- Optimization content
- Performance comparisons
- Statistical data
- Future plans

### Build & Tools

#### Makefile
Build script supporting commands:
- `make build` - Build
- `make test` - Test
- `make coverage` - Coverage
- `make fmt` - Format
- `make vet` - Static check
- `make clean` - Clean
- `make build-all` - Cross-compile

#### validate_dashboard.py
Dashboard validation tool:
- JSON format validation
- Structure integrity check
- Statistics output

#### final_check.sh
Final validation script:
- File integrity check
- Code quality check
- Test execution
- Build verification

### CI/CD

#### .github/workflows/ci.yml
Continuous integration configuration:
- Code format check
- Static analysis
- Unit tests
- Coverage reports

#### .github/workflows/release.yml
Automated release configuration:
- Cross-compilation
- Create Release
- Upload build artifacts

## 📊 Statistics

### Code Volume
- Main program: 1,668 lines
- Test code: 275 lines
- Total code: 1,943 lines

### File Count
- Total files: 25+
- Go source files: 2
- Configuration files: 8
- Documentation files: 7
- Script files: 4

### Monitoring Metrics
- Prometheus metrics: 40+
- Grafana panels: 21 (updated)
- Alert rules: 15 (updated)

### Test Coverage
- Test functions: 8
- Test cases: 38
- Benchmark tests: 3

## 🔄 Development Workflow

### 1. Clone Project
```bash
git clone <repository-url>
cd Vsftpd-exporter
```

### 2. Install Dependencies
```bash
go mod download
```

### 3. Development
```bash
# Edit code
vim vsftp-exporter.go

# Format
make fmt

# Static check
make vet

# Run tests
make test
```

### 4. Build
```bash
make build
```

### 5. Run
```bash
./vsftp-exporter -config=./config.json
```

### 6. Commit
```bash
git add .
git commit -m "feat: add new feature"
git push
```

## 📦 Dependencies

### Direct Dependencies
- `github.com/jlaffaye/ftp` v0.2.0 - FTP client
- `github.com/prometheus/client_golang` v1.19.1 - Prometheus client
- `golang.org/x/crypto` v0.43.0 - SSH support

### Indirect Dependencies
- `github.com/beorn7/perks` v1.0.1
- `github.com/cespare/xxhash/v2` v2.2.0
- `github.com/prometheus/client_model` v0.5.0
- `github.com/prometheus/common` v0.48.0
- `github.com/prometheus/procfs` v0.12.0
- `google.golang.org/protobuf` v1.33.0

## 🎯 Key Features

### Monitoring Capabilities
- ✅ FTP connection status monitoring
- ✅ File transfer statistics
- ✅ User activity tracking
- ✅ Client connection analysis
- ✅ Error and exception monitoring
- ✅ Performance metrics collection

### Technical Features
- ✅ Concurrency safety
- ✅ Incremental log reading
- ✅ SSH remote support
- ✅ Log rotation detection
- ✅ Graceful shutdown
- ✅ Health checks

### Deployment Features
- ✅ Docker containerization
- ✅ Docker Compose one-command deployment
- ✅ Systemd service support
- ✅ Cross-platform compilation
- ✅ CI/CD automation

## 🔗 Related Links

- [Project Home](README.md)
- [Quick Start](QUICKSTART.md)
- [Deployment Guide](DEPLOYMENT.md)
- [Development Guidelines](AGENTS.md)
- [Change Log](CHANGELOG.md)
- [Optimization Report](OPTIMIZATION_SUMMARY.md)
- [Grafana Guide](GRAFANA_DASHBOARD.md)

---

**Last Updated**: 2025-11-29  
**Project Version**: v1.0.0-1 (Dashboard v2.0)
