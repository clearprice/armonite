import React, { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import { apiCall, apiCallSafe, formatTimestamp, getConnectionStatus, debounce } from '../utils/api'

const Dashboard = () => {
  const [coordinatorStatus, setCoordinatorStatus] = useState(null)
  const [agents, setAgents] = useState([])
  const [testRuns, setTestRuns] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [connectionStatus, setConnectionStatus] = useState('connected')
  const intervalRef = useRef(null)
  const isActiveRef = useRef(true)

  const fetchData = async (isInitial = false) => {
    if (!isActiveRef.current) return
    
    try {
      if (isInitial) {
        setError(null)
        
        // Fetch coordinator status
        const status = await apiCall('/status')
        setCoordinatorStatus(status)

        // Fetch agents
        const agentsData = await apiCall('/agents')
        setAgents(agentsData.agents || [])

        // Fetch test runs
        const testRunsData = await apiCall('/test-runs')
        setTestRuns(testRunsData.test_runs || [])
        
        setLoading(false)
      } else {
        // Use safe API calls for polling
        const status = await apiCallSafe('/status')
        const agentsData = await apiCallSafe('/agents')
        const testRunsData = await apiCallSafe('/test-runs')
        
        if (status) setCoordinatorStatus(status)
        if (agentsData) setAgents(agentsData.agents || [])
        if (testRunsData) setTestRuns(testRunsData.test_runs || [])
        
        if (status && agentsData && testRunsData) {
          setError(null)
        }
      }
      setConnectionStatus(getConnectionStatus())
    } catch (err) {
      if (isInitial) {
        setError('Unable to load dashboard data')
        console.error('Failed to fetch dashboard data:', err)
        setLoading(false)
      }
      setConnectionStatus(getConnectionStatus())
    }
  }
  
  const debouncedFetchData = debounce(() => fetchData(false), 500)

  useEffect(() => {
    isActiveRef.current = true
    fetchData(true)
    
    const startPolling = () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
      
      const pollInterval = connectionStatus === 'connected' ? 4000 : 10000
      intervalRef.current = setInterval(() => {
        if (isActiveRef.current) {
          debouncedFetchData()
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
          <h1>Dashboard</h1>
          <p>Overview of your distributed load testing environment</p>
        </div>
        <div className="card">
          <p>Loading dashboard data...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div>
        <div className="header">
          <h1>Dashboard</h1>
          <p>Overview of your distributed load testing environment</p>
        </div>
        <div className="card">
          <p style={{ color: '#666' }}>{error}</p>
        </div>
      </div>
    )
  }

  // Group agents by region
  const agentsByRegion = agents.reduce((acc, agent) => {
    const region = agent.region || 'unknown'
    acc[region] = (acc[region] || 0) + 1
    return acc
  }, {})

  const recentTestRuns = testRuns.slice(-5).reverse()

  return (
    <div>
      <div className="header">
        <h1>Dashboard</h1>
        <p>Overview of your distributed load testing environment</p>
      </div>

      <div className="card">
        <h2>Coordinator Status</h2>
        {coordinatorStatus && (
          <table className="table">
            <tbody>
              <tr><td><strong>Status</strong></td><td>{coordinatorStatus.status}</td></tr>
              <tr><td><strong>Uptime</strong></td><td>{coordinatorStatus.uptime}</td></tr>
              <tr><td><strong>Connected Agents</strong></td><td>{coordinatorStatus.connected_agents}</td></tr>
              <tr><td><strong>Total Test Runs</strong></td><td>{coordinatorStatus.total_test_runs}</td></tr>
              <tr><td><strong>NATS Port</strong></td><td>{coordinatorStatus.nats_port}</td></tr>
              <tr><td><strong>HTTP Port</strong></td><td>{coordinatorStatus.http_port}</td></tr>
            </tbody>
          </table>
        )}
      </div>

      <div className="card">
        <h2>Quick Actions</h2>
        <div style={{ display: 'flex', gap: '16px', marginTop: '16px' }}>
          <Link to="/test-runs" className="btn">Create Test Run</Link>
          <Link to="/agents" className="btn btn-outline">View Agents</Link>
          <Link to="/results" className="btn btn-outline">View Results</Link>
        </div>
      </div>

      <div className="grid-2">
        <div className="card">
          <h2>Connected Agents</h2>
          {agents.length > 0 ? (
            <table className="table">
              <tbody>
                {Object.entries(agentsByRegion).map(([region, count]) => (
                  <tr key={region}>
                    <td>{region}</td>
                    <td>{count} agents</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p>No agents connected</p>
          )}
        </div>

        <div className="card">
          <h2>Current Test Run</h2>
          {coordinatorStatus?.current_test_run ? (
            <div>
              <p><strong>Name:</strong> {coordinatorStatus.current_test_run.name}</p>
              <p><strong>Status:</strong> <span className={`status status-${coordinatorStatus.current_test_run.status}`}>{coordinatorStatus.current_test_run.status}</span></p>
              <p><strong>ID:</strong> <code>{coordinatorStatus.current_test_run.id}</code></p>
              <div style={{ marginTop: '12px' }}>
                <Link to={`/test-runs`} className="btn btn-outline">View Details</Link>
              </div>
            </div>
          ) : (
            <p>No active test run</p>
          )}
        </div>
      </div>

      <div className="card">
        <h2>Recent Test Runs</h2>
        {recentTestRuns.length > 0 ? (
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {recentTestRuns.map(test => (
                <tr key={test.id}>
                  <td>{test.name}</td>
                  <td><span className={`status status-${test.status}`}>{test.status}</span></td>
                  <td>{formatTimestamp(test.created_at)}</td>
                  <td><Link to="/test-runs" className="btn btn-outline">View</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p>No test runs yet. <Link to="/test-runs">Create your first test run</Link></p>
        )}
      </div>
    </div>
  )
}

export default Dashboard