# Armonite Concepts

This document provides in-depth explanations of Armonite's core concepts, architecture patterns, and design principles.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Core Components](#core-components)
- [Message-Driven Design](#message-driven-design)
- [Ramp-up Strategies](#ramp-up-strategies)
- [Load Testing Concepts](#load-testing-concepts)
- [Performance Characteristics](#performance-characteristics)
- [Deployment Patterns](#deployment-patterns)
- [Best Practices](#best-practices)

## Overview

Armonite is a distributed load testing platform designed around modern architectural principles:

- **Message-driven architecture** using NATS for communication
- **Reactive design** with non-blocking operations
- **Horizontal scalability** through agent distribution
- **Real-time telemetry** with minimal overhead
- **Flexible test orchestration** with sophisticated ramp-up strategies

## Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────────────────┐
│                         Armonite Platform                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐                 ┌─────────────────────┐    │
│  │   Coordinator   │◄────────────────┤      Agents         │    │
│  │                 │                 │                     │    │
│  │  ┌───────────┐  │   NATS Messages │  ┌─────────────────┐│    │
│  │  │ Web UI    │  │◄────────────────┤  │ Load Generators ││    │
│  │  └───────────┘  │                 │  └─────────────────┘│    │
│  │  ┌───────────┐  │                 │  ┌─────────────────┐│    │
│  │  │ REST API  │  │                 │  │ HTTP Clients    ││    │
│  │  └───────────┘  │                 │  └─────────────────┘│    │
│  │  ┌───────────┐  │                 │  ┌─────────────────┐│    │
│  │  │ Database  │  │                 │  │ Telemetry       ││    │
│  │  └───────────┘  │                 │  └─────────────────┘│    │
│  └─────────────────┘                 └─────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Communication Flow

1. **Test Creation**: User creates test runs via Web UI or REST API
2. **Agent Registration**: Agents connect to coordinator via NATS
3. **Test Orchestration**: Coordinator broadcasts test plans to agents
4. **Load Generation**: Agents execute HTTP requests according to ramp-up strategy
5. **Telemetry Collection**: Real-time metrics flow back to coordinator
6. **Result Aggregation**: Coordinator processes and stores test results

## Core Components

### Coordinator

The coordinator is the central orchestration component responsible for:

**Responsibilities:**
- Test run lifecycle management
- Agent registration and heartbeat monitoring
- Test plan distribution and orchestration
- Real-time telemetry aggregation
- Result storage and retrieval
- Web UI and REST API serving

**Key Features:**
- Embedded NATS server for high-performance messaging
- SQLite database for persistent storage
- Message-driven architecture with isolated goroutines
- Real-time agent status monitoring
- Comprehensive REST API

### Agents

Agents are distributed load generators that:

**Responsibilities:**
- Connect to coordinator via NATS messaging
- Execute HTTP load tests according to test plans
- Implement ramp-up strategies for gradual load application
- Report real-time telemetry and metrics
- Handle connection pooling and keep-alive optimization

**Key Features:**
- Configurable concurrency levels
- Support for multiple HTTP methods and complex payloads
- Built-in connection pooling and timeout management
- Regional deployment support
- Automatic coordinator discovery and reconnection

### Database Layer

Armonite uses SQLite for persistent storage:

**Data Models:**
- **Test Runs**: Complete test configuration and metadata
- **Agent Results**: Per-agent performance metrics and statistics
- **Test Results**: Aggregated performance data and analysis

**Benefits:**
- Zero-configuration embedded database
- ACID transactions for data consistency
- Efficient querying for large result sets
- Simple backup and portability

## Message-Driven Design

### NATS Messaging Architecture

Armonite uses NATS as its messaging backbone, providing:

**Subject Hierarchy:**
```
armonite.
├── test.command          # Test start/stop commands
├── agent.register        # Agent registration
├── agent.heartbeat       # Agent health monitoring
├── agent.execution       # Agent execution status updates
├── telemetry            # Real-time performance metrics
└── coordinator.internal  # Internal coordinator messages
```

**Message Flow Patterns:**

1. **Command Pattern**: Coordinator → Agents (test commands)
2. **Event Pattern**: Agents → Coordinator (telemetry, status)
3. **Request-Reply Pattern**: API queries for real-time data
4. **Publish-Subscribe Pattern**: Broadcast test plans to multiple agents

### Benefits of Message-Driven Architecture

- **Scalability**: Add agents without coordinator reconfiguration
- **Fault Tolerance**: Automatic reconnection and message retry
- **Performance**: Non-blocking message processing
- **Decoupling**: Components communicate only through well-defined messages

## Ramp-up Strategies

### Strategy Types

#### 1. Immediate Ramp-up
```yaml
ramp_up_strategy:
  type: "immediate"
  duration: "0s"
```

**Characteristics:**
- All agents start at full concurrency immediately
- Maximum load applied from test start
- Useful for stress testing and capacity verification
- May overwhelm target systems without proper preparation

#### 2. Linear Ramp-up
```yaml
ramp_up_strategy:
  type: "linear"
  duration: "60s"
```

**Characteristics:**
- Gradual, linear increase from 0 to full concurrency
- Smooth load curve over specified duration
- Ideal for understanding system behavior under increasing load
- Helps identify performance degradation points

#### 3. Step Ramp-up
```yaml
ramp_up_strategy:
  type: "step"
  duration: "90s"
```

**Characteristics:**
- Predefined steps with equal duration
- Allows system stabilization between load increases
- Good for identifying specific load thresholds
- Default: 3 equal steps over total duration

#### 4. Custom Ramp-up
```yaml
ramp_up_strategy:
  type: "custom"
  duration: "5m"
  phases:
    - duration: "1m"
      concurrency: 25
      mode: "parallel"
    - duration: "2m"
      concurrency: 100
      mode: "parallel"
    - duration: "2m"
      concurrency: 200
      mode: "sequential"
```

**Characteristics:**
- Complete control over load progression
- Variable phase durations and concurrency levels
- Support for parallel and sequential execution modes
- Ideal for complex load testing scenarios

### Execution Modes

**Parallel Mode:**
- All agents execute requests simultaneously
- Maximum throughput and realistic user simulation
- Best for testing concurrent user scenarios

**Sequential Mode:**
- Agents execute requests in coordinated sequence
- Useful for testing serialized operations
- Helps identify bottlenecks in sequential workflows

## Load Testing Concepts

### Virtual Users vs. Requests per Second

**Virtual Users (Concurrency):**
- Simulates real user behavior with think time
- Each virtual user maintains session state
- More realistic for user experience testing
- Better for testing authentication and session management

**Requests per Second (RPS):**
- Focus on raw throughput capability
- No think time between requests
- Better for API and microservice testing
- Easier to correlate with infrastructure metrics

### Think Time

Think time represents the delay between user actions:

```yaml
endpoints:
  - method: "GET"
    url: "https://api.example.com/products"
    think_time: "2s"  # User reads product list for 2 seconds
  - method: "GET" 
    url: "https://api.example.com/product/123"
    think_time: "5s"  # User examines product details for 5 seconds
```

**Benefits:**
- More realistic user behavior simulation
- Prevents overwhelming target systems
- Allows for better resource utilization analysis
- Helps identify caching effectiveness

### Connection Management

**Keep-Alive Connections:**
```bash
./armonite agent --keep-alive=true --concurrency 100
```

**Benefits:**
- Reduced connection overhead
- Better performance for HTTP/1.1
- More realistic browser behavior
- Lower resource usage on both client and server

**Connection Pooling:**
- Automatic management of HTTP connection pools
- Configurable max connections per agent
- Optimal resource utilization
- Reduced latency for subsequent requests

## Performance Characteristics

### Scalability Metrics

**Agent Scalability:**
- Single agent: 1,000-5,000 concurrent users (depending on hardware)
- Multi-agent: Virtually unlimited (tested with 100+ agents)
- Regional distribution: Supports global load generation

**Coordinator Scalability:**
- Handles 100+ agents simultaneously
- Processes 10,000+ telemetry messages/second
- SQLite database scales to millions of test results
- Web UI remains responsive under high load

### Resource Requirements

**Coordinator:**
- CPU: 2-4 cores (more for higher agent counts)
- Memory: 1-4 GB (depending on result history)
- Network: 100 Mbps (for large agent fleets)
- Storage: 10 GB+ (for test result storage)

**Agent:**
- CPU: 1-2 cores per 1,000 concurrent users
- Memory: 512 MB - 2 GB (depending on concurrency)
- Network: Proportional to test load generated
- Storage: Minimal (agents are stateless)

### Performance Optimization

**Coordinator Optimization:**
- Message-driven architecture eliminates blocking operations
- Dedicated goroutines for each component
- Async database operations
- Connection pooling and timeouts

**Agent Optimization:**
- HTTP connection pooling
- Keep-alive connection reuse
- Configurable timeout values
- Memory-efficient request generation

## Deployment Patterns

### Single Machine Development

```bash
# Start coordinator
./armonite coordinator --ui

# Start multiple agents on same machine
./armonite agent --concurrency 100 --id agent-1 &
./armonite agent --concurrency 100 --id agent-2 &
./armonite agent --concurrency 100 --id agent-3 &
```

**Use Cases:**
- Development and testing
- Small-scale load testing
- Proof of concept validation

### Multi-Machine Production

```bash
# Coordinator on dedicated server
./armonite coordinator --host 0.0.0.0 --ui

# Agents distributed across multiple servers
# Server 1
./armonite agent --master-host coordinator.internal --concurrency 500 --region us-east-1

# Server 2  
./armonite agent --master-host coordinator.internal --concurrency 500 --region us-west-2

# Server 3
./armonite agent --master-host coordinator.internal --concurrency 500 --region eu-west-1
```

**Use Cases:**
- Large-scale load testing
- Geographic distribution testing
- High-availability scenarios

### Container Orchestration

**Docker Compose:**
```yaml
version: '3.8'
services:
  coordinator:
    image: armonite:latest
    command: ["coordinator", "--ui", "--host", "0.0.0.0"]
    ports:
      - "8080:8080"
      - "8081:8081"
      - "4222:4222"
    
  agent:
    image: armonite:latest
    command: ["agent", "--master-host", "coordinator", "--concurrency", "200"]
    deploy:
      replicas: 5
    depends_on:
      - coordinator
```

**Kubernetes:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: armonite-agents
spec:
  replicas: 10
  selector:
    matchLabels:
      app: armonite-agent
  template:
    spec:
      containers:
      - name: agent
        image: armonite:latest
        args: ["agent", "--master-host", "armonite-coordinator", "--concurrency", "300"]
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 1Gi
```

### Cloud Deployment

**AWS Auto Scaling Groups:**
- Coordinator on dedicated EC2 instance
- Agent Auto Scaling Groups in multiple regions
- Application Load Balancer for coordinator access
- CloudWatch monitoring for auto-scaling triggers

**Google Kubernetes Engine:**
- Coordinator as StatefulSet with persistent storage
- Agent deployments with horizontal pod autoscaling
- Regional node pools for geographic distribution
- Workload Identity for secure service communication

## Best Practices

### Test Design

1. **Start Small**: Begin with low concurrency and short duration
2. **Gradual Scaling**: Use ramp-up strategies to avoid shocking target systems
3. **Realistic Data**: Use varied test data to avoid artificial caching benefits
4. **Think Time**: Include realistic delays between user actions
5. **Multiple Scenarios**: Test different user flows and load patterns

### Infrastructure

1. **Resource Monitoring**: Monitor both testing infrastructure and target systems
2. **Network Isolation**: Use dedicated networks for load testing when possible
3. **Regional Distribution**: Distribute agents geographically for realistic testing
4. **Capacity Planning**: Ensure adequate resources for planned load levels
5. **Cleanup Procedures**: Implement proper cleanup after test completion

### Operational

1. **Baseline Establishment**: Establish performance baselines before making changes
2. **Incremental Testing**: Test incrementally as system capacity grows
3. **Documentation**: Document test scenarios and expected outcomes
4. **Alerting**: Set up monitoring and alerting for test infrastructure
5. **Result Analysis**: Analyze results in context of business requirements

### Security

1. **Network Security**: Use secure communication channels in production
2. **Authentication**: Implement proper authentication for coordinator access
3. **Data Protection**: Ensure test data doesn't contain sensitive information
4. **Resource Limits**: Implement proper resource limits to prevent abuse
5. **Access Control**: Restrict access to load testing infrastructure

## Advanced Topics

### Custom Metrics

Extend Armonite with custom metrics collection:

```go
type CustomMetrics struct {
    BusinessMetrics map[string]float64 `json:"business_metrics"`
    UserFlowMetrics map[string]int64   `json:"user_flow_metrics"`
}
```

### Integration Patterns

**CI/CD Integration:**
```bash
# Automated performance testing in CI pipeline
./armonite coordinator --plan regression-test.yaml --wait-for-completion
if [ $? -eq 0 ]; then
  echo "Performance test passed"
else
  echo "Performance regression detected"
  exit 1
fi
```

**Monitoring Integration:**
- Prometheus metrics export
- Grafana dashboard integration
- PagerDuty alerting for performance regressions
- Slack notifications for test completion

### Troubleshooting

**Common Issues:**

1. **Agent Connection Problems**
   - Network connectivity issues
   - Firewall configuration
   - DNS resolution problems

2. **Performance Bottlenecks**
   - Resource constraints on agents
   - Network bandwidth limitations
   - Target system capacity limits

3. **Test Reliability Issues**
   - Inadequate ramp-up periods
   - Resource contention between agents
   - Target system instability

**Debugging Techniques:**

1. **Logging Analysis**
   - Enable debug logging for detailed information
   - Analyze coordinator and agent logs separately
   - Look for patterns in error messages

2. **Metrics Analysis**
   - Monitor CPU, memory, and network usage
   - Analyze request/response patterns
   - Identify performance bottlenecks

3. **Network Analysis**
   - Use network monitoring tools
   - Check for packet loss or high latency
   - Verify bandwidth availability

This concepts document provides the foundational understanding needed to effectively use Armonite for distributed load testing. For specific implementation details, refer to the main README and API documentation.