# Armonite

A modern, distributed load testing platform built with Go and React that enables scalable performance testing across multiple agents.

## ✨ Features

- **🚀 Distributed Architecture**: Scale testing across multiple agents in different regions
- **📊 Real-time Monitoring**: Live metrics and telemetry during test execution
- **⚡ Flexible Ramp-up Strategies**: Control how load is applied with immediate, linear, step, and custom ramp-up patterns
- **🎯 Web Interface**: Modern React-based UI for creating and managing test runs
- **💾 SQLite Storage**: Persistent storage for test runs, results, and agent data
- **🔌 HTTP/REST API**: Full REST API for programmatic test management
- **⚡ NATS Messaging**: High-performance messaging between coordinator and agents

## 🚀 Quick Start

### Prerequisites

- Go 1.19 or later
- Node.js 16+ (for building the UI)
- NATS server (embedded)

### Installation

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd armonite
   ```

2. **Build the application**:
   ```bash
   go build -o armonite .
   ```

3. **Build the UI** (optional, for custom UI):
   ```bash
   cd ui-react
   npm install
   npm run build
   cp -r dist/* ../ui-build/
   cd ..
   ```

### Running Armonite

1. **Start the Coordinator**:
   ```bash
   ./armonite coordinator --ui
   ```
   
   The coordinator will start:
   - NATS server on port 4222
   - HTTP API on port 8080
   - Web UI on port 8081

2. **Start Agents** (in separate terminals or machines):
   ```bash
   ./armonite agent --master-host localhost --concurrency 100
   ```

3. **Access the Web UI**:
   Open http://localhost:8081 in your browser

## 📋 Configuration

The system uses a YAML configuration file (`armonite.yaml`):

```yaml
server:
  host: 0.0.0.0
  port: 4222      # NATS communication port
  http_port: 8080 # HTTP API port
  enable_ui: true # Enable web UI

database:
  dsn: "./armonite.db"
  max_open: 25
  max_idle: 5
  max_lifetime: "1h"

defaults:
  concurrency: 100
  duration: 1m
  keep_alive: true
```

## 🎯 Creating Test Runs

### Via Web UI

1. Navigate to the **Test Runs** page
2. Click **Create New Test Run**
3. Configure your test:
   - **Name**: Descriptive name for your test
   - **Duration**: How long to run (e.g., `5m`, `30s`, `1h`)
   - **Concurrency**: Number of virtual users per agent
   - **Target URL**: The endpoint to test
   - **HTTP Method**: GET, POST, PUT, DELETE, PATCH
   - **Headers**: JSON object with request headers
   - **Body**: JSON request body (for non-GET requests)
   - **Ramp-up Strategy**: How to apply load over time

### Via API

```bash
curl -X POST http://localhost:8080/api/v1/test-runs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Load Test",
    "test_plan": {
      "name": "API Load Test",
      "duration": "5m",
      "concurrency": 100,
      "ramp_up_strategy": {
        "type": "linear",
        "duration": "30s"
      },
      "endpoints": [{
        "method": "GET",
        "url": "https://api.example.com/health",
        "headers": {
          "Authorization": "Bearer token"
        }
      }]
    },
    "min_agents": 1
  }'
```

### Via YAML Test Plans

Create a test plan file (`test-plan.yaml`):

```yaml
name: "API Performance Test"
duration: "10m"
concurrency: 200
ramp_up_strategy:
  type: "custom"
  duration: "2m"
  phases:
    - duration: "30s"
      concurrency: 50
      mode: "parallel"
    - duration: "30s"
      concurrency: 100
      mode: "parallel"
    - duration: "1m"
      concurrency: 200
      mode: "parallel"
endpoints:
  - method: "POST"
    url: "https://api.example.com/users"
    headers:
      Content-Type: "application/json"
      Authorization: "Bearer your-token"
    body:
      name: "Test User"
      email: "test@example.com"
```

## ⚡ Ramp-up Strategies

Control how load is applied over time:

### Immediate
All agents start at full concurrency immediately:
```yaml
ramp_up_strategy:
  type: "immediate"
  duration: "0s"
```

### Linear
Gradually increase load over time:
```yaml
ramp_up_strategy:
  type: "linear" 
  duration: "60s"  # Reach full load over 1 minute
```

### Step
Increase load in predefined steps:
```yaml
ramp_up_strategy:
  type: "step"
  duration: "90s"  # 3 steps over 90 seconds
```

### Custom
Define specific phases:
```yaml
ramp_up_strategy:
  type: "custom"
  duration: "2m"
  phases:
    - duration: "30s"
      concurrency: 25
      mode: "parallel"
    - duration: "30s" 
      concurrency: 75
      mode: "parallel"
    - duration: "60s"
      concurrency: 150
      mode: "sequential"
```

## 🛠 CLI Commands

### Coordinator Commands

```bash
# Start coordinator with web UI
./armonite coordinator --ui

# Start coordinator on custom ports
./armonite coordinator --port 4223 --http-port 8081

# Start coordinator with minimum agents requirement
./armonite coordinator --min-agents 3
```

### Agent Commands

```bash
# Start agent with default settings
./armonite agent

# Start agent with custom configuration
./armonite agent \
  --master-host coordinator.example.com \
  --master-port 4222 \
  --concurrency 200 \
  --region us-east-1 \
  --id agent-001

# Start agent with keep-alive disabled
./armonite agent --keep-alive=false
```

## 🔌 API Reference

### Test Runs

- `GET /api/v1/test-runs` - List all test runs
- `POST /api/v1/test-runs` - Create a new test run  
- `GET /api/v1/test-runs/{id}` - Get test run details
- `POST /api/v1/test-runs/{id}/start` - Start a test run
- `POST /api/v1/test-runs/{id}/stop` - Stop a running test
- `POST /api/v1/test-runs/{id}/rerun` - Rerun a completed test
- `GET /api/v1/test-runs/{id}/results` - Get test results
- `DELETE /api/v1/test-runs/{id}` - Delete a test run

### Coordinator Status

- `GET /api/v1/status` - Get coordinator status
- `GET /api/v1/agents` - List connected agents
- `GET /health` - Health check endpoint

### Utilities

- `POST /api/v1/test-connection` - Test endpoint connectivity

## 🏗 Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Browser   │    │   Agent (N)     │    │   Agent (N)     │
│                 │    │                 │    │                 │
│  React UI       │    │  Load Generator │    │  Load Generator │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │ HTTP API              │ NATS                  │ NATS
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Coordinator                                  │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │ HTTP Server │  │ NATS Server │  │   Message Handlers      │ │
│  │             │  │             │  │                         │ │
│  │ REST API    │  │ Agent Comm  │  │ - Telemetry Processing  │ │
│  │ Web UI      │  │ Messaging   │  │ - Agent Registration    │ │
│  └─────────────┘  └─────────────┘  │ - Test Orchestration    │ │
│                                    └─────────────────────────┘ │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  SQLite Database                        │   │
│  │  - Test Runs    - Agent Results    - Configuration     │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## 📊 Monitoring and Metrics

### Real-time Metrics

During test execution, view live metrics:
- **Requests per second**
- **Average response time**
- **Error rate**
- **HTTP status code distribution**
- **Agent status and performance**

### Test Results

After test completion:
- **Total requests and errors**
- **Success rate percentage**
- **Latency statistics (min, max, avg)**
- **Per-agent breakdown**
- **Timeline analysis**

## 🎯 Best Practices

### Agent Deployment

1. **Multiple Regions**: Deploy agents across different geographical regions
2. **Resource Allocation**: Ensure adequate CPU and memory for high concurrency
3. **Network Configuration**: Verify connectivity between agents and coordinator
4. **Monitoring**: Monitor agent health and performance

### Test Design

1. **Gradual Ramp-up**: Use ramp-up strategies to avoid overwhelming target systems
2. **Realistic Load Patterns**: Design tests that mimic real user behavior
3. **Test Data**: Use varied test data to avoid caching effects
4. **Duration**: Run tests long enough to identify performance trends

### Performance Tuning

1. **Connection Keep-alive**: Enable for better performance with HTTP/1.1
2. **Concurrency Limits**: Start with lower concurrency and scale up
3. **Think Time**: Add realistic delays between requests
4. **Resource Monitoring**: Monitor both testing infrastructure and target systems

## 🐛 Troubleshooting

### Common Issues

**Agent not connecting to coordinator:**
- Verify network connectivity on port 4222
- Check firewall settings
- Ensure coordinator is running

**High memory usage:**
- Reduce agent concurrency
- Implement connection pooling
- Monitor for memory leaks in target application

**Connection timeouts:**
- Increase timeout values in configuration
- Check network latency between agents and targets
- Verify target system capacity

**UI not accessible:**
- Ensure coordinator is started with `--ui` flag
- Check if port 8081 is available
- Verify ui-build directory exists

### Logs and Debugging

Enable debug logging:
```yaml
logging:
  level: debug
  format: json
```

View coordinator logs for detailed information about:
- Agent connections and disconnections
- Test execution status
- Message processing
- Database operations

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🆘 Support

For support and questions:
- Create an issue on GitHub
- Check the documentation:
  - [Concepts Guide](CONCEPTS.md) - Architecture and design principles
  - [Configuration Guide](CONFIG.md) - Complete configuration reference
- Review existing issues and discussions