import React, { useState, useEffect, useRef } from 'react'
import { apiCall, apiCallSafe, formatTimestamp, getConnectionStatus, debounce } from '../utils/api'

const Results = () => {
  const [testRuns, setTestRuns] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [selectedTestRun, setSelectedTestRun] = useState(null)
  const [showDetailModal, setShowDetailModal] = useState(false)
  const [detailData, setDetailData] = useState(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState('connected')
  const intervalRef = useRef(null)
  const isActiveRef = useRef(true)

  const fetchResults = async (isInitial = false) => {
    if (!isActiveRef.current) return
    
    try {
      if (isInitial) {
        setError(null)
        const data = await apiCall('/test-runs')
        const completedTests = (data.test_runs || []).filter(test => 
          test.status === 'completed' && test.results
        )
        setTestRuns(completedTests)
        setLoading(false)
      } else {
        const data = await apiCallSafe('/test-runs')
        if (data && data.test_runs) {
          const completedTests = data.test_runs.filter(test => 
            test.status === 'completed' && test.results
          )
          setTestRuns(completedTests)
          setError(null)
        }
      }
      setConnectionStatus(getConnectionStatus())
    } catch (err) {
      if (isInitial) {
        setError('Unable to load results data')
        console.error('Failed to fetch results:', err)
        setLoading(false)
      }
      setConnectionStatus(getConnectionStatus())
    }
  }
  
  const debouncedFetchResults = debounce(() => fetchResults(false), 1000)

  useEffect(() => {
    isActiveRef.current = true
    fetchResults(true)
    
    const startPolling = () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
      
      // Less frequent polling for results since they don't change as often
      const pollInterval = connectionStatus === 'connected' ? 10000 : 15000
      intervalRef.current = setInterval(() => {
        if (isActiveRef.current) {
          debouncedFetchResults()
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

  const rerunTestRun = async (testRunId) => {
    try {
      await apiCall(`/test-runs/${testRunId}/rerun`, { method: 'POST' })
      // Optionally refresh results or show a success message
    } catch (err) {
      console.error('Failed to rerun test:', err)
    }
  }

  const deleteTestRun = async (testRunId, testName) => {
    if (!window.confirm(`Are you sure you want to delete the test run "${testName}"? This action cannot be undone.`)) {
      return
    }

    try {
      await apiCall(`/test-runs/${testRunId}`, { method: 'DELETE' })
      // Refresh the results list to remove the deleted test
      fetchResults(true)
    } catch (err) {
      console.error('Failed to delete test run:', err)
      alert('Failed to delete test run. Please try again.')
    }
  }

  const viewDetailedResults = async (testRun) => {
    setSelectedTestRun(testRun)
    setShowDetailModal(true)
    setLoadingDetail(true)
    setDetailData(null)
    
    try {
      const data = await apiCall(`/test-runs/${testRun.id}/results`)
      setDetailData(data)
    } catch (err) {
      console.error('Failed to fetch detailed results:', err)
      setDetailData({ error: 'Failed to load detailed results' })
    } finally {
      setLoadingDetail(false)
    }
  }

  if (loading) {
    return (
      <div>
        <div className="header">
          <h1>Results</h1>
          <p>Analysis and insights from completed test runs</p>
        </div>
        <div className="card">
          <p>Loading results data...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div>
        <div className="header">
          <h1>Results</h1>
          <p>Analysis and insights from completed test runs</p>
        </div>
        <div className="card">
          <p style={{ color: '#666' }}>{error}</p>
        </div>
      </div>
    )
  }

  if (testRuns.length === 0) {
    return (
      <div>
        <div className="header">
          <h1>Results</h1>
          <p>Analysis and insights from completed test runs</p>
        </div>
        <div className="card">
          <div style={{ textAlign: 'center', padding: '40px', color: '#666' }}>
            <p style={{ fontSize: '18px', marginBottom: '16px' }}>No completed test runs</p>
            <p>Run some tests to see results and analytics here.</p>
          </div>
        </div>
      </div>
    )
  }

  // Calculate aggregate statistics
  const totalRequests = testRuns.reduce((sum, test) => sum + (test.results?.total_requests || 0), 0)
  const totalErrors = testRuns.reduce((sum, test) => sum + (test.results?.total_errors || 0), 0)
  const avgSuccessRate = testRuns.reduce((sum, test) => sum + (test.results?.success_rate || 0), 0) / testRuns.length
  const avgLatency = testRuns.reduce((sum, test) => sum + (test.results?.avg_latency_ms || 0), 0) / testRuns.length

  return (
    <div>
      <div className="header">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h1>Results</h1>
            <p>Analysis and insights from completed test runs</p>
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
        <h2>Overall Statistics</h2>
        <div className="grid-4" style={{ marginBottom: '20px' }}>
          <div className="stat-card">
            <div className="stat-value">{testRuns.length}</div>
            <div className="stat-label">Completed Tests</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{totalRequests.toLocaleString()}</div>
            <div className="stat-label">Total Requests</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{avgSuccessRate.toFixed(1)}%</div>
            <div className="stat-label">Avg Success Rate</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{avgLatency.toFixed(1)}ms</div>
            <div className="stat-label">Avg Latency</div>
          </div>
        </div>
      </div>

      <div className="card">
        <h2>Test Run Results</h2>
        <table className="table">
          <thead>
            <tr>
              <th>Test Name</th>
              <th>Completed</th>
              <th>Duration</th>
              <th>Requests</th>
              <th>Errors</th>
              <th>Success Rate</th>
              <th>Avg Latency</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {testRuns.map(test => (
              <tr key={test.id}>
                <td>
                  <strong>{test.name}</strong><br/>
                  <small><code>{test.id}</code></small>
                </td>
                <td>{test.completed_at ? formatTimestamp(test.completed_at) : 'N/A'}</td>
                <td>{test.duration || 'N/A'}</td>
                <td>{(test.results?.total_requests || 0).toLocaleString()}</td>
                <td>{(test.results?.total_errors || 0).toLocaleString()}</td>
                <td>{(test.results?.success_rate || 0).toFixed(1)}%</td>
                <td>{(test.results?.avg_latency_ms || 0).toFixed(2)}ms</td>
                <td>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <button className="btn btn-outline" onClick={() => viewDetailedResults(test)}>View Details</button>
                    <button className="btn" onClick={() => rerunTestRun(test.id)}>Rerun</button>
                    <button className="btn btn-danger" onClick={() => deleteTestRun(test.id, test.name)}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Detailed Results Modal */}
      {showDetailModal && selectedTestRun && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.5)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1000
        }}>
          <div style={{
            backgroundColor: 'white',
            borderRadius: '8px',
            padding: '24px',
            width: '90%',
            maxWidth: '900px',
            maxHeight: '80vh',
            overflow: 'auto'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
              <h2>Detailed Results: {selectedTestRun.name}</h2>
              <button 
                onClick={() => {
                  setShowDetailModal(false)
                  setSelectedTestRun(null)
                  setDetailData(null)
                }}
                style={{ background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer' }}
              >
                ×
              </button>
            </div>

            {loadingDetail ? (
              <p>Loading detailed results...</p>
            ) : detailData && detailData.error ? (
              <p style={{ color: '#d32f2f' }}>{detailData.error}</p>
            ) : detailData ? (
              <div>
                <div className="grid-4" style={{ marginBottom: '20px' }}>
                  <div className="stat-card">
                    <div className="stat-value">{(detailData.summary?.total_requests || 0).toLocaleString()}</div>
                    <div className="stat-label">Total Requests</div>
                  </div>
                  <div className="stat-card">
                    <div className="stat-value">{(detailData.summary?.total_errors || 0).toLocaleString()}</div>
                    <div className="stat-label">Total Errors</div>
                  </div>
                  <div className="stat-card">
                    <div className="stat-value">{(detailData.summary?.success_rate || 0).toFixed(1)}%</div>
                    <div className="stat-label">Success Rate</div>
                  </div>
                  <div className="stat-card">
                    <div className="stat-value">{(detailData.summary?.avg_latency_ms || 0).toFixed(1)}ms</div>
                    <div className="stat-label">Avg Latency</div>
                  </div>
                </div>

                {detailData.summary && (
                  <div className="grid-4" style={{ marginBottom: '20px' }}>
                    <div className="stat-card">
                      <div className="stat-value">{(detailData.summary.requests_per_sec || 0).toFixed(1)}</div>
                      <div className="stat-label">Requests/sec</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-value">{(detailData.summary.min_latency_ms || 0).toFixed(1)}ms</div>
                      <div className="stat-label">Min Latency</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-value">{(detailData.summary.max_latency_ms || 0).toFixed(1)}ms</div>
                      <div className="stat-label">Max Latency</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-value">{detailData.agent_results?.length || 0}</div>
                      <div className="stat-label">Active Agents</div>
                    </div>
                  </div>
                )}

                <div style={{ marginBottom: '20px' }}>
                  <h3>Test Configuration</h3>
                  <div className="code">
                    <strong>Duration:</strong> {selectedTestRun.duration || 'N/A'}<br/>
                    <strong>Target URL:</strong> {detailData.test_run?.test_plan?.endpoints?.[0]?.url || 'N/A'}<br/>
                    <strong>Method:</strong> {detailData.test_run?.test_plan?.endpoints?.[0]?.method || 'N/A'}<br/>
                    <strong>Concurrency:</strong> {detailData.test_run?.test_plan?.concurrency || 'N/A'}<br/>
                    <strong>Agent Count:</strong> {detailData.test_run?.agent_count || 'N/A'}
                  </div>
                </div>

                {detailData.summary?.status_codes && Object.keys(detailData.summary.status_codes).length > 0 && (
                  <div style={{ marginBottom: '20px' }}>
                    <h3>HTTP Status Codes</h3>
                    <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
                      {Object.entries(detailData.summary.status_codes).map(([code, count]) => (
                        <div key={code} className="stat-card" style={{ minWidth: '120px' }}>
                          <div className="stat-value">{count.toLocaleString()}</div>
                          <div className="stat-label">HTTP {code}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {detailData.agent_results && detailData.agent_results.length > 0 && (
                  <div>
                    <h3>Agent Performance</h3>
                    <table className="table">
                      <thead>
                        <tr>
                          <th>Agent ID</th>
                          <th>Requests</th>
                          <th>Errors</th>
                          <th>Success Rate</th>
                          <th>Avg Latency</th>
                          <th>Min Latency</th>
                          <th>Max Latency</th>
                        </tr>
                      </thead>
                      <tbody>
                        {detailData.agent_results.map(agent => (
                          <tr key={agent.agent_id}>
                            <td><code>{agent.agent_id}</code></td>
                            <td>{agent.requests.toLocaleString()}</td>
                            <td>{agent.errors.toLocaleString()}</td>
                            <td>{agent.requests > 0 ? ((agent.requests - agent.errors) / agent.requests * 100).toFixed(1) : '0.0'}%</td>
                            <td>{agent.avg_latency_ms.toFixed(1)}ms</td>
                            <td>{(agent.min_latency_ms || 0).toFixed(1)}ms</td>
                            <td>{(agent.max_latency_ms || 0).toFixed(1)}ms</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            ) : (
              <p>No detailed results available</p>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '24px', gap: '12px' }}>
              <button className="btn" onClick={() => rerunTestRun(selectedTestRun.id)}>Rerun Test</button>
              <button className="btn btn-danger" onClick={() => {
                deleteTestRun(selectedTestRun.id, selectedTestRun.name)
                setShowDetailModal(false)
                setSelectedTestRun(null)
                setDetailData(null)
              }}>Delete Test</button>
              <button 
                className="btn btn-outline"
                onClick={() => {
                  setShowDetailModal(false)
                  setSelectedTestRun(null)
                  setDetailData(null)
                }}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default Results