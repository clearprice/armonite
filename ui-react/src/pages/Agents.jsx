import React, { useState, useEffect, useRef } from 'react'
import { apiCall, apiCallSafe, formatTimestamp, getConnectionStatus, debounce } from '../utils/api'

const Agents = () => {
  const [agents, setAgents] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [connectionStatus, setConnectionStatus] = useState('connected')
  const intervalRef = useRef(null)
  const isActiveRef = useRef(true)

  const getExecutionStateClass = (state) => {
    switch (state) {
      case 'running':
      case 'starting':
        return 'status-busy'
      case 'stopping':
        return 'status-waiting'
      case 'idle':
      case 'completed':
        return 'status-online'
      default:
        return 'status-online'
    }
  }

  const getConnectionStatusClass = (isStale) => {
    return isStale ? 'status-error' : 'status-online'
  }

  const getExecutionStateDisplay = (state) => {
    if (state === 'running' || state === 'starting') {
      return (
        <>
          <div className="spinner"></div>
          {state}
        </>
      )
    }
    return state || 'idle'
  }

  const fetchAgents = async (isInitial = false) => {
    if (!isActiveRef.current) return
    
    try {
      if (isInitial) {
        setError(null)
        const data = await apiCall('/agents')
        setAgents(data.agents || [])
        setLoading(false)
      } else {
        // Use safe API call for polling to avoid UI disruption
        const data = await apiCallSafe('/agents')
        if (data && data.agents) {
          setAgents(data.agents)
          setError(null)
        }
      }
      setConnectionStatus(getConnectionStatus())
    } catch (err) {
      if (isInitial) {
        setError('Unable to load agents data')
        console.error('Failed to fetch agents:', err)
        setLoading(false)
      }
      setConnectionStatus(getConnectionStatus())
    }
  }
  
  // Debounced version for rapid updates
  const debouncedFetchAgents = debounce(() => fetchAgents(false), 500)

  useEffect(() => {
    isActiveRef.current = true
    fetchAgents(true)
    
    // Dynamic polling based on connection status
    const startPolling = () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
      
      const pollInterval = connectionStatus === 'connected' ? 3000 : 8000
      intervalRef.current = setInterval(() => {
        if (isActiveRef.current) {
          debouncedFetchAgents()
        }
      }, pollInterval)
    }
    
    startPolling()
    
    return () => {
      isActiveRef.current = false
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
      }
    }
  }, [connectionStatus])

  if (loading) {
    return (
      <div>
        <div className="header">
          <h1>Agents</h1>
          <p>Monitor and manage connected load testing agents</p>
        </div>
        <div className="card">
          <h2>Agent Status</h2>
          <p>Loading agents data...</p>
        </div>
      </div>
    )
  }

  const totalConcurrency = agents.reduce((sum, agent) => sum + agent.concurrency, 0)
  const totalRequests = agents.reduce((sum, agent) => sum + (agent.requests || 0), 0)
  const totalErrors = agents.reduce((sum, agent) => sum + (agent.errors || 0), 0)

  return (
    <div>
      <div className="header">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h1>Agents</h1>
            <p>Monitor and manage connected load testing agents</p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span className={`status status-${connectionStatus === 'connected' ? 'online' : connectionStatus === 'reconnecting' ? 'busy' : 'error'}`}>
              {connectionStatus === 'reconnecting' && <div className="spinner"></div>}
              {connectionStatus}
            </span>
          </div>
        </div>
      </div>

      <div className="card">
        <h2>Agent Status</h2>
        {error ? (
          <p style={{ color: '#666' }}>{error}</p>
        ) : agents.length > 0 ? (
          <div>
            <div className="grid-4" style={{ marginBottom: '20px' }}>
              <div className="stat-card">
                <div className="stat-value">{agents.length}</div>
                <div className="stat-label">Total Agents</div>
              </div>
              <div className="stat-card">
                <div className="stat-value">{totalConcurrency}</div>
                <div className="stat-label">Total Concurrency</div>
              </div>
              <div className="stat-card">
                <div className="stat-value">{totalRequests.toLocaleString()}</div>
                <div className="stat-label">Total Requests</div>
              </div>
              <div className="stat-card">
                <div className="stat-value">{totalErrors.toLocaleString()}</div>
                <div className="stat-label">Total Errors</div>
              </div>
            </div>

            <table className="table">
              <thead>
                <tr>
                  <th>Agent ID</th>
                  <th>Region</th>
                  <th>Concurrency</th>
                  <th>Connected</th>
                  <th>Last Seen</th>
                  <th>Requests</th>
                  <th>Errors</th>
                  <th>Avg Latency</th>
                  <th>Connection</th>
                  <th>State</th>
                </tr>
              </thead>
              <tbody>
                {agents.map(agent => {
                  const lastSeen = new Date(agent.last_seen)
                  const now = new Date()
                  const secondsAgo = Math.floor((now - lastSeen) / 1000)
                  const isStale = secondsAgo > 60

                  return (
                    <tr key={agent.id}>
                      <td><code>{agent.id}</code></td>
                      <td>{agent.region || 'N/A'}</td>
                      <td>{agent.concurrency}</td>
                      <td>{formatTimestamp(agent.connected_at)}</td>
                      <td>{formatTimestamp(agent.last_seen)}</td>
                      <td>{agent.requests || 0}</td>
                      <td>{agent.errors || 0}</td>
                      <td>{agent.avg_latency_ms ? agent.avg_latency_ms.toFixed(2) + 'ms' : 'N/A'}</td>
                      <td>
                        <span className={`status ${getConnectionStatusClass(isStale)}`}>
                          {isStale ? 'offline' : 'online'}
                        </span>
                      </td>
                      <td>
                        <span className={`status ${getExecutionStateClass(agent.execution_state || 'idle')}`}>
                          {getExecutionStateDisplay(agent.execution_state)}
                        </span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: '40px', color: '#666' }}>
            <p style={{ fontSize: '18px', marginBottom: '16px' }}>No agents connected</p>
            <p>Start agents using the command below to begin load testing.</p>
          </div>
        )}
      </div>

      <div className="card">
        <h2>Connection Instructions</h2>
        
        <h3 style={{ marginBottom: '12px', fontSize: '16px' }}>Binary/Direct Installation</h3>
        <p style={{ marginBottom: '8px' }}>To connect new agents to this coordinator, run the following command:</p>
        <div className="code">
          ./armonite agent --master-host 0.0.0.0 --master-port 4222 --concurrency 100 --region your-region
        </div>
        
        <h3 style={{ margin: '24px 0 12px 0', fontSize: '16px' }}>Docker</h3>
        <p style={{ marginBottom: '8px' }}>To run an agent using Docker:</p>
        <div className="code">
          {`docker run -d --name armonite-agent \\
  armonite:latest agent \\
  --master-host 0.0.0.0 \\
  --master-port 4222 \\
  --concurrency 100 \\
  --region your-region`}
        </div>
        
        <p style={{ marginTop: '16px', color: '#666', fontSize: '14px' }}>
          <strong>Note:</strong> Replace <code>your-region</code> with a descriptive name (e.g., us-east-1, europe, datacenter-1). 
          Adjust <code>--concurrency</code> based on your agent's capacity.
        </p>
      </div>
    </div>
  )
}

export default Agents