import React, { useState, useEffect, useRef } from 'react'
import { apiCall, apiCallSafe, formatTimestamp, getConnectionStatus, debounce } from '../utils/api'

const TestRuns = () => {
  const [testRuns, setTestRuns] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState(null)
  const [formData, setFormData] = useState({
    name: '',
    duration: '5m',
    concurrency: 50,
    minAgents: 1,
    url: '',
    method: 'GET',
    headers: '{}',
    body: '{}',
    rampUpType: 'immediate',
    rampUpDuration: '30s',
    customPhases: [
      { duration: '10s', concurrency: 25, mode: 'parallel' },
      { duration: '10s', concurrency: 50, mode: 'parallel' },
      { duration: '10s', concurrency: 100, mode: 'parallel' }
    ]
  })
  const [selectedTestRun, setSelectedTestRun] = useState(null)
  const [showResultsModal, setShowResultsModal] = useState(false)
  const [resultsData, setResultsData] = useState(null)
  const [loadingResults, setLoadingResults] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState('connected')
  const intervalRef = useRef(null)
  const isActiveRef = useRef(true)
  const [testingConnection, setTestingConnection] = useState(false)
  const [connectionTestResult, setConnectionTestResult] = useState(null)
  const [showStartConfirmation, setShowStartConfirmation] = useState(false)
  const [testToStart, setTestToStart] = useState(null)
  const [startConfirmationData, setStartConfirmationData] = useState(null)
  const [agents, setAgents] = useState([])
  const [agentsLoading, setAgentsLoading] = useState(true)

  const fetchTestRuns = async (isInitial = false) => {
    if (!isActiveRef.current) return
    
    try {
      if (isInitial) {
        setError(null)
        const data = await apiCall('/test-runs')
        setTestRuns(data.test_runs || [])
        setLoading(false)
      } else {
        const data = await apiCallSafe('/test-runs')
        if (data && data.test_runs) {
          setTestRuns(data.test_runs)
          setError(null)
        }
      }
      setConnectionStatus(getConnectionStatus())
    } catch (err) {
      if (isInitial) {
        setError('Unable to load test runs data')
        console.error('Failed to fetch test runs:', err)
        setLoading(false)
      }
      setConnectionStatus(getConnectionStatus())
    }
  }

  const fetchAgents = async (isInitial = false) => {
    if (!isActiveRef.current) return
    
    try {
      if (isInitial) {
        const data = await apiCall('/agents')
        setAgents(data.agents || [])
        setAgentsLoading(false)
      } else {
        const data = await apiCallSafe('/agents')
        if (data && data.agents) {
          setAgents(data.agents || [])
        }
      }
    } catch (err) {
      if (isInitial) {
        console.error('Failed to fetch agents:', err)
        setAgentsLoading(false)
      }
    }
  }
  
  const debouncedFetchTestRuns = debounce(() => fetchTestRuns(false), 500)
  const debouncedFetchAgents = debounce(() => fetchAgents(false), 500)

  useEffect(() => {
    isActiveRef.current = true
    fetchTestRuns(true)
    fetchAgents(true)
    
    const startPolling = () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
      
      // More frequent polling for test runs since they change status quickly
      const pollInterval = connectionStatus === 'connected' ? 2000 : 8000
      intervalRef.current = setInterval(() => {
        if (isActiveRef.current) {
          debouncedFetchTestRuns()
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

  const createTestRun = async (e) => {
    e.preventDefault()
    setCreating(true)
    setCreateError(null)

    try {
      // Parse headers and body
      let headers = {}
      let body = {}
      
      try {
        headers = JSON.parse(formData.headers)
      } catch (err) {
        throw new Error('Invalid JSON in headers field')
      }

      if (formData.method !== 'GET' && formData.body.trim()) {
        try {
          body = JSON.parse(formData.body)
        } catch (err) {
          throw new Error('Invalid JSON in body field')
        }
      }

      // Build ramp-up strategy
      let rampUpStrategy = null
      if (formData.rampUpType !== 'immediate') {
        rampUpStrategy = {
          type: formData.rampUpType,
          duration: formData.rampUpDuration,
          ...(formData.rampUpType === 'custom' && {
            phases: formData.customPhases.filter(phase => 
              phase.duration && phase.concurrency > 0
            )
          })
        }
      }

      const testRunData = {
        name: formData.name,
        test_plan: {
          name: formData.name,
          duration: formData.duration,
          concurrency: formData.concurrency,
          ...(rampUpStrategy && { ramp_up_strategy: rampUpStrategy }),
          endpoints: [{
            method: formData.method,
            url: formData.url,
            headers: headers,
            ...(formData.method !== 'GET' && formData.body.trim() && { body: body })
          }]
        },
        min_agents: formData.minAgents
      }

      await apiCall('/test-runs', {
        method: 'POST',
        body: JSON.stringify(testRunData)
      })

      // Reset form and close modal
      setFormData({
        name: '',
        duration: '5m',
        concurrency: 50,
        minAgents: 1,
        url: '',
        method: 'GET',
        headers: '{}',
        body: '{}',
        rampUpType: 'immediate',
        rampUpDuration: '30s',
        customPhases: [
          { duration: '10s', concurrency: 25, mode: 'parallel' },
          { duration: '10s', concurrency: 50, mode: 'parallel' },
          { duration: '10s', concurrency: 100, mode: 'parallel' }
        ]
      })
      setConnectionTestResult(null)
      setShowCreateModal(false)
      
      // Refresh test runs
      fetchTestRuns()
      
    } catch (err) {
      setCreateError(err.message)
    } finally {
      setCreating(false)
    }
  }

  const handleInputChange = (field, value) => {
    setFormData(prev => ({ ...prev, [field]: value }))
  }

  const showStartConfirmationDialog = (testRun) => {
    console.log('=== START DIALOG DEBUG ===')
    console.log('Global agents state:', agents)
    console.log('Test run:', testRun)
    console.log('Test run agent_count:', testRun.agent_count)
    
    // Calculate estimated requests using global agents state
    const estimatedData = calculateEstimatedRequests(testRun, agents)
    console.log('Estimated data:', estimatedData)
    console.log('=== END DEBUG ===')
    
    setTestToStart(testRun)
    setStartConfirmationData(estimatedData)
    setShowStartConfirmation(true)
  }

  const calculateEstimatedRequests = (testRun, agents) => {
    console.log('--- CALCULATION DEBUG ---')
    console.log('Input agents:', agents)
    console.log('Agent count in array:', agents.length)
    
    const activeAgents = agents.filter(agent => agent.status === 'connected')
    console.log('Active agents after filter:', activeAgents)
    console.log('Active agent count:', activeAgents.length)
    
    const agentCount = Math.min(testRun.agent_count || 1, activeAgents.length)
    console.log('Final agent count:', agentCount)
    console.log('Test run agent_count:', testRun.agent_count)
    
    // Parse duration
    const durationStr = testRun.test_plan?.duration || '1m'
    const durationMs = parseDuration(durationStr)
    const durationSeconds = durationMs / 1000
    
    // Calculate requests based on ramp-up strategy
    let estimatedRequests = 0
    const concurrency = testRun.test_plan?.concurrency || 50
    
    if (testRun.test_plan?.ramp_up_strategy) {
      const strategy = testRun.test_plan.ramp_up_strategy
      
      if (strategy.type === 'immediate') {
        // Full concurrency for entire duration
        estimatedRequests = concurrency * agentCount * durationSeconds
      } else if (strategy.type === 'linear') {
        // Average concurrency is half of max during ramp-up
        const rampDuration = parseDuration(strategy.duration || '30s') / 1000
        const fullLoadDuration = Math.max(0, durationSeconds - rampDuration)
        const avgConcurrencyDuringRamp = concurrency / 2
        
        estimatedRequests = (avgConcurrencyDuringRamp * rampDuration + concurrency * fullLoadDuration) * agentCount
      } else if (strategy.type === 'custom' && strategy.phases) {
        // Sum up requests for each phase
        strategy.phases.forEach(phase => {
          const phaseDurationSeconds = parseDuration(phase.duration || '10s') / 1000
          const phaseConcurrency = Math.min(phase.concurrency || 10, concurrency)
          estimatedRequests += phaseConcurrency * agentCount * phaseDurationSeconds
        })
      } else {
        // Default fallback
        estimatedRequests = concurrency * agentCount * durationSeconds
      }
    } else {
      // No ramp-up strategy, assume immediate
      estimatedRequests = concurrency * agentCount * durationSeconds
    }
    
    return {
      agents: activeAgents,
      totalAgents: agentCount,
      estimatedRequests: Math.round(estimatedRequests),
      concurrency,
      duration: durationStr,
      durationSeconds: Math.round(durationSeconds)
    }
  }

  const parseDuration = (duration) => {
    const match = duration.match(/^(\d+)([smh])$/)
    if (!match) return 60000 // Default 1 minute
    
    const value = parseInt(match[1])
    const unit = match[2]
    
    switch (unit) {
      case 's': return value * 1000
      case 'm': return value * 60 * 1000
      case 'h': return value * 60 * 60 * 1000
      default: return 60000
    }
  }

  const confirmStartTestRun = async () => {
    if (!testToStart) return
    
    try {
      await apiCall(`/test-runs/${testToStart.id}/start`, { method: 'POST' })
      fetchTestRuns(true)
      setShowStartConfirmation(false)
      setTestToStart(null)
      setStartConfirmationData(null)
    } catch (err) {
      console.error('Failed to start test run:', err)
      alert('Failed to start test run. Please try again.')
    }
  }

  const startTestRun = async (testRunId) => {
    const testRun = testRuns.find(t => t.id === testRunId)
    if (testRun) {
      showStartConfirmationDialog(testRun)
    }
  }

  const stopTestRun = async (testRunId) => {
    try {
      await apiCall(`/test-runs/${testRunId}/stop`, { method: 'POST' })
      fetchTestRuns(true)
    } catch (err) {
      console.error('Failed to stop test run:', err)
    }
  }

  const rerunTestRun = async (testRunId) => {
    try {
      await apiCall(`/test-runs/${testRunId}/rerun`, { method: 'POST' })
      fetchTestRuns(true)
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
      fetchTestRuns(true)
    } catch (err) {
      console.error('Failed to delete test run:', err)
      alert('Failed to delete test run. It may still be running or already deleted.')
    }
  }

  const viewResults = async (testRun) => {
    setSelectedTestRun(testRun)
    setShowResultsModal(true)
    setLoadingResults(true)
    setResultsData(null)
    
    try {
      const data = await apiCall(`/test-runs/${testRun.id}/results`)
      setResultsData(data)
    } catch (err) {
      console.error('Failed to fetch results:', err)
      setResultsData({ error: 'Failed to load results' })
    } finally {
      setLoadingResults(false)
    }
  }

  const testConnection = async () => {
    setTestingConnection(true)
    setConnectionTestResult(null)
    
    try {
      // Parse headers and body
      let headers = {}
      let body = {}
      
      try {
        headers = JSON.parse(formData.headers)
      } catch (err) {
        setConnectionTestResult({
          success: false,
          error: 'Invalid JSON in headers field'
        })
        setTestingConnection(false)
        return
      }

      if (formData.method !== 'GET' && formData.body.trim()) {
        try {
          body = JSON.parse(formData.body)
        } catch (err) {
          setConnectionTestResult({
            success: false,
            error: 'Invalid JSON in body field'
          })
          setTestingConnection(false)
          return
        }
      }

      const testData = {
        url: formData.url,
        method: formData.method,
        headers: headers,
        ...(formData.method !== 'GET' && formData.body.trim() && { body: body })
      }

      const result = await apiCall('/test-connection', {
        method: 'POST',
        body: JSON.stringify(testData)
      })

      setConnectionTestResult(result)
      
    } catch (err) {
      setConnectionTestResult({
        success: false,
        error: err.message || 'Failed to test connection'
      })
    } finally {
      setTestingConnection(false)
    }
  }

  if (loading) {
    return (
      <div>
        <div className="header">
          <h1>Test Runs</h1>
          <p>Create and manage load testing campaigns</p>
        </div>
        <div className="card">
          <p>Loading test runs...</p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="header">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h1>Test Runs</h1>
            <p>Create and manage load testing campaigns</p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span style={{ fontSize: '14px', color: '#666' }}>Agents:</span>
              <span className={`status ${agents.filter(a => a.status === 'connected').length > 0 ? 'status-online' : 'status-offline'}`}>
                {agentsLoading ? '...' : agents.filter(a => a.status === 'connected').length}
              </span>
            </div>
            <span className={`status status-${connectionStatus === 'connected' ? 'online' : connectionStatus === 'reconnecting' ? 'busy' : 'error'}`}>
              {connectionStatus === 'reconnecting' && <div className="spinner"></div>}
              {connectionStatus}
            </span>
          </div>
        </div>
      </div>

      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h2>Test Runs</h2>
          <button className="btn" onClick={() => {
            setShowCreateModal(true)
            setConnectionTestResult(null)
          }}>Create New Test Run</button>
        </div>
        
        {error ? (
          <p style={{ color: '#666' }}>{error}</p>
        ) : testRuns.length > 0 ? (
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Created</th>
                <th>Started</th>
                <th>Duration</th>
                <th>Agents</th>
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
                  <td><span className={`status status-${test.status}`}>{test.status.replace('_', ' ')}</span></td>
                  <td>{formatTimestamp(test.created_at)}</td>
                  <td>{test.started_at ? formatTimestamp(test.started_at) : 'N/A'}</td>
                  <td>{test.duration || 'N/A'}</td>
                  <td>{test.agent_count || 0}</td>
                  <td>
                    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                      {(test.status === 'completed' || test.status === 'failed') && test.results && (
                        <button className="btn btn-outline" onClick={() => viewResults(test)}>View Results</button>
                      )}
                      {test.status === 'created' && <button className="btn" onClick={() => startTestRun(test.id)}>Start</button>}
                      {test.status === 'running' && <button className="btn" onClick={() => stopTestRun(test.id)}>Stop</button>}
                      {(test.status === 'completed' || test.status === 'failed' || test.status === 'cancelled') && (
                        <button className="btn" onClick={() => rerunTestRun(test.id)}>Rerun</button>
                      )}
                      {(test.status === 'completed' || test.status === 'failed' || test.status === 'cancelled') && (
                        <button className="btn btn-danger" onClick={() => deleteTestRun(test.id, test.name)}>Delete</button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div style={{ textAlign: 'center', padding: '40px', color: '#666' }}>
            <p style={{ fontSize: '18px', marginBottom: '16px' }}>No test runs created yet</p>
            <p>Create your first test run to begin load testing.</p>
            <button className="btn" style={{ marginTop: '16px' }} onClick={() => {
              setShowCreateModal(true)
              setConnectionTestResult(null)
            }}>Create Test Run</button>
          </div>
        )}
      </div>

      {/* Create Test Run Modal */}
      {showCreateModal && (
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
            maxWidth: '600px',
            maxHeight: '80vh',
            overflow: 'auto'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
              <h2>Create New Test Run</h2>
              <button 
                onClick={() => {
                  setShowCreateModal(false)
                  setConnectionTestResult(null)
                }}
                style={{ background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer' }}
              >
                ×
              </button>
            </div>

            {createError && (
              <div style={{ padding: '12px', backgroundColor: '#f5f5f5', color: '#d32f2f', marginBottom: '16px', borderRadius: '4px' }}>
                {createError}
              </div>
            )}

            <form onSubmit={createTestRun}>
              <div className="form-group">
                <label className="form-label">Test Run Name *</label>
                <input 
                  type="text"
                  className="form-input"
                  value={formData.name}
                  onChange={(e) => handleInputChange('name', e.target.value)}
                  required
                  placeholder="e.g., API Load Test"
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                <div className="form-group">
                  <label className="form-label">Duration *</label>
                  <input 
                    type="text"
                    className="form-input"
                    value={formData.duration}
                    onChange={(e) => handleInputChange('duration', e.target.value)}
                    required
                    placeholder="e.g., 5m, 30s, 1h"
                  />
                </div>

                <div className="form-group">
                  <label className="form-label">Concurrency *</label>
                  <input 
                    type="number"
                    className="form-input"
                    value={formData.concurrency}
                    onChange={(e) => handleInputChange('concurrency', parseInt(e.target.value))}
                    required
                    min="1"
                  />
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Minimum Agents</label>
                <input 
                  type="number"
                  className="form-input"
                  value={formData.minAgents}
                  onChange={(e) => handleInputChange('minAgents', parseInt(e.target.value))}
                  min="1"
                />
              </div>

              {/* Ramp-up Strategy Section */}
              <div className="form-group">
                <label className="form-label">Ramp-up Strategy</label>
                <select 
                  className="form-input"
                  value={formData.rampUpType}
                  onChange={(e) => handleInputChange('rampUpType', e.target.value)}
                >
                  <option value="immediate">Immediate - Start all workers at once</option>
                  <option value="linear">Linear - Gradually increase over time</option>
                  <option value="step">Step - Increase in phases</option>
                  <option value="custom">Custom - Define specific phases</option>
                </select>
              </div>

              {formData.rampUpType !== 'immediate' && (
                <div className="form-group">
                  <label className="form-label">Ramp-up Duration</label>
                  <input 
                    type="text"
                    className="form-input"
                    value={formData.rampUpDuration}
                    onChange={(e) => handleInputChange('rampUpDuration', e.target.value)}
                    placeholder="e.g., 30s, 1m, 2m"
                  />
                  <small style={{ color: '#666', fontSize: '12px' }}>
                    Time to reach full concurrency ({formData.concurrency} workers)
                  </small>
                </div>
              )}

              {formData.rampUpType === 'custom' && (
                <div className="form-group">
                  <label className="form-label">Custom Phases</label>
                  <div style={{ marginBottom: '12px' }}>
                    {formData.customPhases.map((phase, index) => (
                      <div key={index} style={{ 
                        display: 'grid', 
                        gridTemplateColumns: '1fr 1fr 1fr auto', 
                        gap: '8px', 
                        marginBottom: '8px',
                        alignItems: 'center'
                      }}>
                        <input
                          type="text"
                          className="form-input"
                          placeholder="Duration (e.g., 10s)"
                          value={phase.duration}
                          onChange={(e) => {
                            const newPhases = [...formData.customPhases]
                            newPhases[index].duration = e.target.value
                            handleInputChange('customPhases', newPhases)
                          }}
                        />
                        <input
                          type="number"
                          className="form-input"
                          placeholder="Concurrency"
                          value={phase.concurrency}
                          min="0"
                          max={formData.concurrency}
                          onChange={(e) => {
                            const newPhases = [...formData.customPhases]
                            newPhases[index].concurrency = parseInt(e.target.value) || 0
                            handleInputChange('customPhases', newPhases)
                          }}
                        />
                        <select
                          className="form-input"
                          value={phase.mode}
                          onChange={(e) => {
                            const newPhases = [...formData.customPhases]
                            newPhases[index].mode = e.target.value
                            handleInputChange('customPhases', newPhases)
                          }}
                        >
                          <option value="parallel">Parallel</option>
                          <option value="sequential">Sequential</option>
                        </select>
                        <button
                          type="button"
                          onClick={() => {
                            if (formData.customPhases.length > 1) {
                              const newPhases = formData.customPhases.filter((_, i) => i !== index)
                              handleInputChange('customPhases', newPhases)
                            }
                          }}
                          disabled={formData.customPhases.length <= 1}
                          style={{
                            background: 'none',
                            border: '1px solid #ddd',
                            borderRadius: '4px',
                            width: '32px',
                            height: '32px',
                            cursor: formData.customPhases.length > 1 ? 'pointer' : 'not-allowed',
                            color: formData.customPhases.length > 1 ? '#666' : '#ccc'
                          }}
                        >
                          ✕
                        </button>
                      </div>
                    ))}
                    <button
                      type="button"
                      onClick={() => {
                        const newPhases = [...formData.customPhases, { duration: '10s', concurrency: 50, mode: 'parallel' }]
                        handleInputChange('customPhases', newPhases)
                      }}
                      className="btn btn-outline"
                      style={{ fontSize: '14px', padding: '8px 16px', marginTop: '8px' }}
                    >
                      Add Phase
                    </button>
                  </div>
                  <small style={{ color: '#666', fontSize: '12px' }}>
                    Define how agents ramp up their load over time. Each phase specifies duration, target concurrency, and execution mode.
                  </small>
                </div>
              )}

              <div className="form-group">
                <label className="form-label">Target URL *</label>
                <input 
                  type="url"
                  className="form-input"
                  value={formData.url}
                  onChange={(e) => handleInputChange('url', e.target.value)}
                  required
                  placeholder="https://api.example.com/endpoint"
                />
              </div>

              <div className="form-group">
                <label className="form-label">HTTP Method</label>
                <select 
                  className="form-input"
                  value={formData.method}
                  onChange={(e) => handleInputChange('method', e.target.value)}
                >
                  <option value="GET">GET</option>
                  <option value="POST">POST</option>
                  <option value="PUT">PUT</option>
                  <option value="DELETE">DELETE</option>
                  <option value="PATCH">PATCH</option>
                </select>
              </div>

              <div className="form-group">
                <label className="form-label">Headers (JSON)</label>
                <textarea 
                  className="form-input"
                  value={formData.headers}
                  onChange={(e) => handleInputChange('headers', e.target.value)}
                  rows="3"
                  placeholder='{"Content-Type": "application/json", "Authorization": "Bearer token"}'
                />
              </div>

              {formData.method !== 'GET' && (
                <div className="form-group">
                  <label className="form-label">Request Body (JSON)</label>
                  <textarea 
                    className="form-input"
                    value={formData.body}
                    onChange={(e) => handleInputChange('body', e.target.value)}
                    rows="4"
                    placeholder='{"key": "value"}'
                  />
                </div>
              )}

              {/* Test Connection Section */}
              <div className="form-group">
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
                  <label className="form-label">Test Connection</label>
                  <button 
                    type="button"
                    className="btn btn-outline"
                    onClick={testConnection}
                    disabled={testingConnection || !formData.url}
                    style={{ fontSize: '14px', padding: '8px 16px' }}
                  >
                    {testingConnection ? 'Testing...' : 'Test Connection'}
                  </button>
                </div>
                
                {connectionTestResult && (
                  <div style={{
                    padding: '12px',
                    borderRadius: '4px',
                    backgroundColor: connectionTestResult.success ? '#e8f5e8' : '#f5e8e8',
                    border: `1px solid ${connectionTestResult.success ? '#4caf50' : '#f44336'}`,
                    marginTop: '8px'
                  }}>
                    <div style={{ 
                      display: 'flex', 
                      alignItems: 'center', 
                      marginBottom: connectionTestResult.success ? '8px' : '0'
                    }}>
                      <span style={{
                        color: connectionTestResult.success ? '#2e7d32' : '#d32f2f',
                        fontWeight: 'bold',
                        marginRight: '8px'
                      }}>
                        {connectionTestResult.success ? '✓' : '✗'}
                      </span>
                      <span style={{
                        color: connectionTestResult.success ? '#2e7d32' : '#d32f2f',
                        fontWeight: '500'
                      }}>
                        {connectionTestResult.success ? 'Connection successful' : 'Connection failed'}
                      </span>
                    </div>
                    
                    {connectionTestResult.success ? (
                      <div style={{ fontSize: '14px', color: '#666' }}>
                        <div><strong>Status:</strong> {connectionTestResult.status_code}</div>
                        <div><strong>Response time:</strong> {connectionTestResult.response_time_ms?.toFixed(0)}ms</div>
                        {connectionTestResult.body_preview && (
                          <div style={{ marginTop: '8px' }}>
                            <strong>Response preview:</strong>
                            <div className="code" style={{ marginTop: '4px', fontSize: '12px', maxHeight: '60px', overflow: 'auto' }}>
                              {connectionTestResult.body_preview}
                            </div>
                          </div>
                        )}
                      </div>
                    ) : (
                      <div style={{ fontSize: '14px', color: '#d32f2f' }}>
                        {connectionTestResult.error}
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end', marginTop: '24px' }}>
                <button 
                  type="button"
                  className="btn btn-outline"
                  onClick={() => {
                    setShowCreateModal(false)
                    setConnectionTestResult(null)
                  }}
                  disabled={creating}
                >
                  Cancel
                </button>
                <button 
                  type="submit"
                  className="btn"
                  disabled={creating}
                >
                  {creating ? 'Creating...' : 'Create Test Run'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Results Modal */}
      {showResultsModal && selectedTestRun && (
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
            maxWidth: '800px',
            maxHeight: '80vh',
            overflow: 'auto'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
              <h2>Test Results: {selectedTestRun.name}</h2>
              <button 
                onClick={() => {
                  setShowResultsModal(false)
                  setSelectedTestRun(null)
                  setResultsData(null)
                }}
                style={{ background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer' }}
              >
                ×
              </button>
            </div>

            {loadingResults ? (
              <p>Loading results...</p>
            ) : resultsData && resultsData.error ? (
              <p style={{ color: '#d32f2f' }}>{resultsData.error}</p>
            ) : resultsData ? (
              <div>
                <div className="grid-4" style={{ marginBottom: '20px' }}>
                  <div className="stat-card">
                    <div className="stat-value">{(resultsData.summary?.total_requests || 0).toLocaleString()}</div>
                    <div className="stat-label">Total Requests</div>
                  </div>
                  <div className="stat-card">
                    <div className="stat-value">{(resultsData.summary?.total_errors || 0).toLocaleString()}</div>
                    <div className="stat-label">Total Errors</div>
                  </div>
                  <div className="stat-card">
                    <div className="stat-value">{(resultsData.summary?.success_rate || 0).toFixed(1)}%</div>
                    <div className="stat-label">Success Rate</div>
                  </div>
                  <div className="stat-card">
                    <div className="stat-value">{(resultsData.summary?.avg_latency_ms || 0).toFixed(1)}ms</div>
                    <div className="stat-label">Avg Latency</div>
                  </div>
                </div>

                {resultsData.summary?.requests_per_sec && (
                  <div className="grid-4" style={{ marginBottom: '20px' }}>
                    <div className="stat-card">
                      <div className="stat-value">{resultsData.summary.requests_per_sec.toFixed(1)}</div>
                      <div className="stat-label">Requests/sec</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-value">{(resultsData.summary.min_latency_ms || 0).toFixed(1)}ms</div>
                      <div className="stat-label">Min Latency</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-value">{(resultsData.summary.max_latency_ms || 0).toFixed(1)}ms</div>
                      <div className="stat-label">Max Latency</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-value">{resultsData.agent_results?.length || 0}</div>
                      <div className="stat-label">Active Agents</div>
                    </div>
                  </div>
                )}

                {resultsData.summary?.status_codes && Object.keys(resultsData.summary.status_codes).length > 0 && (
                  <div style={{ marginBottom: '20px' }}>
                    <h3>HTTP Status Codes</h3>
                    <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
                      {Object.entries(resultsData.summary.status_codes).map(([code, count]) => (
                        <div key={code} className="stat-card" style={{ minWidth: '120px' }}>
                          <div className="stat-value">{count.toLocaleString()}</div>
                          <div className="stat-label">HTTP {code}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {resultsData.agent_results && resultsData.agent_results.length > 0 && (
                  <div>
                    <h3>Agent Results</h3>
                    <table className="table">
                      <thead>
                        <tr>
                          <th>Agent ID</th>
                          <th>Requests</th>
                          <th>Errors</th>
                          <th>Success Rate</th>
                          <th>Avg Latency</th>
                        </tr>
                      </thead>
                      <tbody>
                        {resultsData.agent_results.map(agent => (
                          <tr key={agent.agent_id}>
                            <td><code>{agent.agent_id}</code></td>
                            <td>{agent.requests.toLocaleString()}</td>
                            <td>{agent.errors.toLocaleString()}</td>
                            <td>{((agent.requests - agent.errors) / agent.requests * 100).toFixed(1)}%</td>
                            <td>{agent.avg_latency_ms.toFixed(1)}ms</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            ) : (
              <p>No results data available</p>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '24px' }}>
              <button 
                className="btn btn-outline"
                onClick={() => {
                  setShowResultsModal(false)
                  setSelectedTestRun(null)
                  setResultsData(null)
                }}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Start Test Confirmation Modal */}
      {showStartConfirmation && testToStart && startConfirmationData && (
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
            maxWidth: '600px',
            maxHeight: '80vh',
            overflow: 'auto'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
              <h2>⚠️ Confirm Test Start</h2>
              <button 
                onClick={() => {
                  setShowStartConfirmation(false)
                  setTestToStart(null)
                  setStartConfirmationData(null)
                }}
                style={{ background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer' }}
              >
                ×
              </button>
            </div>

            <div style={{ marginBottom: '20px' }}>
              <h3>Test Plan Summary</h3>
              <div className="code" style={{ marginBottom: '16px' }}>
                <strong>Name:</strong> {testToStart.name}<br/>
                <strong>Target URL:</strong> {testToStart.test_plan?.endpoints?.[0]?.url || 'N/A'}<br/>
                <strong>Method:</strong> {testToStart.test_plan?.endpoints?.[0]?.method || 'GET'}<br/>
                <strong>Duration:</strong> {startConfirmationData.duration}<br/>
                <strong>Concurrency:</strong> {startConfirmationData.concurrency} per agent<br/>
                <strong>Ramp-up:</strong> {testToStart.test_plan?.ramp_up_strategy?.type || 'immediate'}
                {testToStart.test_plan?.ramp_up_strategy?.type === 'linear' && (
                  <span> ({testToStart.test_plan.ramp_up_strategy.duration})</span>
                )}
              </div>
            </div>

            <div style={{ marginBottom: '20px' }}>
              <h3>Load Estimation</h3>
              <div className="grid-4" style={{ marginBottom: '16px' }}>
                <div className="stat-card">
                  <div className="stat-value">{startConfirmationData.totalAgents}</div>
                  <div className="stat-label">Active Agents</div>
                </div>
                <div className="stat-card">
                  <div className="stat-value">{startConfirmationData.concurrency}</div>
                  <div className="stat-label">Concurrency/Agent</div>
                </div>
                <div className="stat-card">
                  <div className="stat-value">{startConfirmationData.durationSeconds}s</div>
                  <div className="stat-label">Duration</div>
                </div>
                <div className="stat-card">
                  <div className="stat-value" style={{ color: '#ff6b35' }}>
                    {startConfirmationData.estimatedRequests.toLocaleString()}
                  </div>
                  <div className="stat-label">Est. Requests</div>
                </div>
              </div>
              
              <div style={{ 
                padding: '16px', 
                backgroundColor: '#fff3cd', 
                border: '1px solid #ffeaa7', 
                borderRadius: '4px',
                marginBottom: '16px'
              }}>
                <strong>⚠️ Load Warning:</strong> This test will generate approximately{' '}
                <strong>{startConfirmationData.estimatedRequests.toLocaleString()} requests</strong> to the target server.
                Make sure the target can handle this load and you have permission to test it.
              </div>

              {startConfirmationData.totalAgents === 0 && (
                <div style={{ 
                  padding: '16px', 
                  backgroundColor: '#f8d7da', 
                  border: '1px solid #f5c6cb', 
                  borderRadius: '4px',
                  marginBottom: '16px'
                }}>
                  <strong>❌ No Active Agents:</strong> There are no online agents available to run this test.
                  Please start at least one agent before running the test.
                </div>
              )}
            </div>

            {testToStart.test_plan?.ramp_up_strategy?.type === 'custom' && 
             testToStart.test_plan.ramp_up_strategy.phases && (
              <div style={{ marginBottom: '20px' }}>
                <h3>Custom Phases</h3>
                <table className="table" style={{ fontSize: '14px' }}>
                  <thead>
                    <tr>
                      <th>Phase</th>
                      <th>Duration</th>
                      <th>Concurrency</th>
                      <th>Mode</th>
                      <th>Est. Requests</th>
                    </tr>
                  </thead>
                  <tbody>
                    {testToStart.test_plan.ramp_up_strategy.phases.map((phase, index) => {
                      const phaseDuration = parseDuration(phase.duration || '10s') / 1000
                      const phaseConcurrency = Math.min(phase.concurrency || 10, startConfirmationData.concurrency)
                      const phaseRequests = phaseConcurrency * startConfirmationData.totalAgents * phaseDuration
                      
                      return (
                        <tr key={index}>
                          <td>Phase {index + 1}</td>
                          <td>{phase.duration}</td>
                          <td>{phaseConcurrency}</td>
                          <td>{phase.mode}</td>
                          <td>{Math.round(phaseRequests).toLocaleString()}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
              <button 
                className="btn btn-outline"
                onClick={() => {
                  setShowStartConfirmation(false)
                  setTestToStart(null)
                  setStartConfirmationData(null)
                }}
              >
                Cancel
              </button>
              <button 
                className="btn"
                onClick={confirmStartTestRun}
                disabled={startConfirmationData.totalAgents === 0}
                style={{
                  opacity: startConfirmationData.totalAgents === 0 ? 0.5 : 1,
                  cursor: startConfirmationData.totalAgents === 0 ? 'not-allowed' : 'pointer'
                }}
              >
                {startConfirmationData.totalAgents === 0 ? 'No Agents Available' : 'Start Test'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default TestRuns