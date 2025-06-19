// Use window.location to dynamically determine API URL
const getApiBaseUrl = () => {
  const protocol = window.location.protocol
  const hostname = window.location.hostname
  // UI server runs on API port + 1, so API is on port - 1
  const apiPort = parseInt(window.location.port) - 1
  return `${protocol}//${hostname}:${apiPort}/api/v1`
}

const API_BASE_URL = getApiBaseUrl()

class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.status = status
  }
}

// Connection state management
let connectionRetries = 0
const maxRetries = 3

// Request deduplication to prevent duplicate concurrent requests
const pendingRequests = new Map()

const createRequestKey = (endpoint, options) => {
  return `${options.method || 'GET'}_${endpoint}_${JSON.stringify(options.body || {})}`
}

export const apiCall = async (endpoint, options = {}) => {
  // Check for existing pending request
  const requestKey = createRequestKey(endpoint, options)
  if (pendingRequests.has(requestKey)) {
    return pendingRequests.get(requestKey)
  }
  
  const requestPromise = (async () => {
    try {
      const controller = new AbortController()
      const timeout = options.timeout || 5000
      const timeoutId = setTimeout(() => controller.abort(), timeout)
      
      const response = await fetch(API_BASE_URL + endpoint, {
        headers: {
          'Content-Type': 'application/json',
          ...options.headers
        },
        signal: controller.signal,
        ...options
      })
      
      clearTimeout(timeoutId)
      
      if (!response.ok) {
        throw new ApiError(`HTTP ${response.status}: ${response.statusText}`, response.status)
      }
      
      // Reset retry counter on successful request
      connectionRetries = 0
      
      return await response.json()
    } catch (error) {
      connectionRetries++
      
      if (error.name === 'AbortError') {
        throw new ApiError('Request timed out', 408)
      }
      
      // For network errors, implement exponential backoff
      if (connectionRetries > maxRetries) {
        throw new ApiError('Connection lost after multiple retries', 503)
      }
      
      throw error
    } finally {
      // Clean up pending request
      pendingRequests.delete(requestKey)
    }
  })()
  
  // Store pending request
  pendingRequests.set(requestKey, requestPromise)
  
  return requestPromise
}

// Safe API call with built-in error handling for polling
export const apiCallSafe = async (endpoint, options = {}) => {
  try {
    return await apiCall(endpoint, { ...options, timeout: 3000 })
  } catch (error) {
    console.warn(`API call failed for ${endpoint}:`, error.message)
    return null
  }
}

// Get connection status
export const getConnectionStatus = () => {
  return connectionRetries === 0 ? 'connected' : connectionRetries > maxRetries ? 'disconnected' : 'reconnecting'
}

// Utility functions
export const formatDuration = (seconds) => {
  if (seconds < 60) return seconds + 's'
  if (seconds < 3600) return Math.round(seconds / 60) + 'm'
  return Math.round(seconds / 3600) + 'h'
}

export const formatTimestamp = (timestamp) => {
  return new Date(timestamp).toLocaleString()
}

// Debounce function for reducing API calls
export const debounce = (func, wait) => {
  let timeout
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout)
      func(...args)
    }
    clearTimeout(timeout)
    timeout = setTimeout(later, wait)
  }
}