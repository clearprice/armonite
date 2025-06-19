import React, { useState, useEffect } from 'react'
import { apiCall } from '../utils/api'

const Settings = () => {
  const [coordinatorStatus, setCoordinatorStatus] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const status = await apiCall('/status')
        setCoordinatorStatus(status)
      } catch (err) {
        console.error('Failed to fetch coordinator status:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchStatus()
  }, [])

  if (loading) {
    return (
      <div>
        <div className="header">
          <h1>Settings</h1>
          <p>Configuration and system information</p>
        </div>
        <div className="card">
          <p>Loading settings...</p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="header">
        <h1>Settings</h1>
        <p>Configuration and system information</p>
      </div>

      <div className="card">
        <h2>System Information</h2>
        {coordinatorStatus && (
          <table className="table">
            <tbody>
              <tr><td><strong>Service</strong></td><td>armonite-coordinator</td></tr>
              <tr><td><strong>Host</strong></td><td>{coordinatorStatus.host}</td></tr>
              <tr><td><strong>HTTP Port</strong></td><td>{coordinatorStatus.http_port}</td></tr>
              <tr><td><strong>NATS Port</strong></td><td>{coordinatorStatus.nats_port}</td></tr>
              <tr><td><strong>Status</strong></td><td>{coordinatorStatus.status}</td></tr>
              <tr><td><strong>Uptime</strong></td><td>{coordinatorStatus.uptime}</td></tr>
            </tbody>
          </table>
        )}
      </div>

      <div className="card">
        <h2>API Documentation</h2>
        <h3>Coordinator Endpoints</h3>
        <div className="code">
GET /api/v1/status      - Get coordinator status
GET /api/v1/agents      - List connected agents
GET /api/v1/metrics     - Get performance metrics
GET /health             - Health check
        </div>

        <h3 style={{ marginTop: '24px' }}>Test Run Management</h3>
        <div className="code">
POST   /api/v1/test-runs        - Create new test run
GET    /api/v1/test-runs        - List all test runs
GET    /api/v1/test-runs/:id    - Get specific test run
POST   /api/v1/test-runs/:id/start - Start test run
POST   /api/v1/test-runs/:id/stop  - Stop test run
DELETE /api/v1/test-runs/:id    - Delete test run
        </div>

        <h3 style={{ marginTop: '24px' }}>Example: Create Test Run</h3>
        <div className="code">
{`curl -X POST http://localhost:8050/api/v1/test-runs \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "API Load Test",
    "test_plan": {
      "name": "Basic API Test",
      "duration": "5m",
      "concurrency": 50,
      "endpoints": [
        {
          "method": "GET",
          "url": "https://api.example.com/health",
          "headers": {"User-Agent": "Armonite/1.0"}
        }
      ]
    },
    "min_agents": 2
  }'`}
        </div>
      </div>

      <div className="card">
        <h2>Agent Configuration</h2>
        <p>Agents can be configured with the following parameters:</p>
        <div className="code">
--master-host     Coordinator hostname/IP
--master-port     Coordinator NATS port (default: 4222)
--concurrency     Number of concurrent requests per agent
--region          Agent region identifier
--think-time      Delay between requests (default: 0)
        </div>
      </div>
    </div>
  )
}

export default Settings