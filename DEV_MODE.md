# Development Mode Documentation

## Overview

Armonite agents can consume significant system resources during load testing, which is expected behavior for production testing. However, during development and testing of Armonite itself, you may want controlled resource usage to prevent system overload.

The `--dev` flag enables development mode with sensible resource limits.

## Development Mode Features

### 🚦 Automatic Resource Limits
When `--dev` is enabled, the agent applies these defaults:

| Setting | Production Default | Dev Mode Default | Description |
|---------|-------------------|------------------|-------------|
| **Concurrency** | Unlimited | **100** | Maximum concurrent connections |
| **Rate Limit** | Unlimited | **1000 req/s** | Maximum requests per second |
| **Think Time** | 0ms | **200ms** | Delay between requests |

### 🎛️ Manual Override Options
You can still override dev mode defaults with explicit flags:

```bash
# Custom rate limiting
./armonite agent --dev --rate-limit 500

# Custom think time
./armonite agent --dev --default-think-time 500ms

# Custom concurrency (still limited to 100 in dev mode)
./armonite agent --dev --concurrency 50
```

## Usage Examples

### Basic Development Mode
```bash
# Start agent with all dev mode defaults
./armonite agent --master-host localhost --dev

# Equivalent to:
./armonite agent --master-host localhost \
  --concurrency 100 \
  --rate-limit 1000 \
  --default-think-time 200ms
```

### Development Mode with Custom Settings
```bash
# Lower rate limit for gentler testing
./armonite agent --master-host localhost --dev --rate-limit 100

# Longer think time for more realistic user simulation
./armonite agent --master-host localhost --dev --default-think-time 1s

# Combination of custom settings
./armonite agent --master-host localhost --dev \
  --rate-limit 250 \
  --default-think-time 300ms \
  --concurrency 50
```

### Production Mode (No Limits)
```bash
# Full production mode - no resource limits
./armonite agent --master-host localhost --concurrency 1000

# Custom production settings
./armonite agent --master-host localhost \
  --concurrency 500 \
  --rate-limit 5000
```

## Makefile Targets

### Quick Development Testing
```bash
# Run agent in dev mode
make run-agent-dev MASTER_HOST=localhost

# Run regular agent (production mode)
make run-agent MASTER_HOST=localhost
```

## How Rate Limiting Works

### Rate Limiter Implementation
- Uses a token bucket approach with a buffered channel
- Fills at the specified rate (requests per second)
- Workers wait for tokens before making requests
- Non-blocking - skips if channel is full

### Think Time Behavior
```go
// Endpoint-specific think time takes precedence
endpoint.ThinkTime = "500ms"  // Uses 500ms

// Falls back to agent default if endpoint doesn't specify
agent.defaultThinkTime = 200ms  // Uses 200ms if endpoint.ThinkTime is empty

// No delay if neither is set
// Both are 0 - no think time applied
```

## Impact on Load Testing

### Development Mode Impact
```yaml
# Test plan with dev mode agent
name: "Dev Mode Test"
duration: "1m"
endpoints:
  - method: "GET"
    url: "https://api.example.com/health"
    # Agent applies 200ms think time even without this
```

**Effective Rate**: ~300 req/min per agent (1000 req/s limited by 200ms think time)

### Production Mode Impact
```yaml
# Same test plan with production agent
name: "Production Test"
duration: "1m"
endpoints:
  - method: "GET"
    url: "https://api.example.com/health"
```

**Effective Rate**: ~60,000 req/min per agent (limited only by system resources)

## Monitoring Dev Mode

### Log Output
```bash
# Dev mode startup logs
INFO[2024-01-15T10:30:00Z] Development mode enabled - applying resource limits
INFO[2024-01-15T10:30:00Z] Dev mode: Limited concurrency to 100
INFO[2024-01-15T10:30:00Z] Dev mode: Set rate limit to 1000 requests/second  
INFO[2024-01-15T10:30:00Z] Dev mode: Set default think time to 200ms
INFO[2024-01-15T10:30:00Z] Agent agent-1234567890 started, connecting to localhost:4222
INFO[2024-01-15T10:30:00Z] Development mode: Rate limit: 1000 req/s, Default think time: 200ms
```

### Runtime Behavior
- Each worker waits for rate limit token before making requests
- Think time is applied after each request starts
- Rate limiting is per-agent (not global across all agents)

## Use Cases

### 🧪 Armonite Development
```bash
# Test Armonite features without overwhelming your system
./armonite agent --dev --master-host localhost
```

### 🏠 Local API Testing  
```bash
# Test local APIs without aggressive load
./armonite agent --dev --master-host localhost --rate-limit 50
```

### 📚 Learning/Training
```bash
# Educational use with controlled load
./armonite agent --dev --default-think-time 1s --rate-limit 10
```

### 🔍 Debugging
```bash
# Slow, observable requests for debugging
./armonite agent --dev --concurrency 1 --rate-limit 1 --default-think-time 5s
```

## Best Practices

### Development Workflow
1. **Start with dev mode** for initial testing
2. **Graduate to production mode** for actual load testing
3. **Use custom limits** for specific scenarios

### Resource Planning
```bash
# Estimate load with dev mode:
# 100 concurrency × 1000 req/s ÷ 200ms think time = ~500 effective req/s

# Scale up gradually:
./armonite agent --dev --rate-limit 100    # Gentle start
./armonite agent --dev --rate-limit 500    # Medium load  
./armonite agent --dev                     # Full dev load
./armonite agent --concurrency 1000        # Production load
```

### Multi-Agent Testing
```bash
# Terminal 1: Gentle agent
./armonite agent --dev --rate-limit 100 --id agent-gentle

# Terminal 2: Medium agent  
./armonite agent --dev --rate-limit 500 --id agent-medium

# Terminal 3: Full dev agent
./armonite agent --dev --id agent-full
```

## Troubleshooting

### Agent Using Too Many Resources
```bash
# Reduce rate limit
./armonite agent --dev --rate-limit 50

# Increase think time
./armonite agent --dev --default-think-time 1s

# Reduce concurrency
./armonite agent --dev --concurrency 10
```

### Rate Limiting Not Working
- Check that `--dev` flag or `--rate-limit` is specified
- Verify rate limiter logs in startup messages
- Remember: rate limiting is per-agent, not global

### Think Time Not Applied
- Endpoint-specific `think_time` overrides agent default
- Set think_time to empty string in test plan to use agent default
- Check logs for effective think time being applied

## Configuration Examples

### Conservative Development
```bash
./armonite agent --dev \
  --concurrency 10 \
  --rate-limit 50 \
  --default-think-time 1s
```

### Balanced Development  
```bash
./armonite agent --dev \
  --rate-limit 250 \
  --default-think-time 500ms
```

### Aggressive Development
```bash
./armonite agent --dev
# Uses all dev mode defaults
```

### Production Ready
```bash
./armonite agent \
  --concurrency 1000 \
  --rate-limit 10000
# No artificial limits
```