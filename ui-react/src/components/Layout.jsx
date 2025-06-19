import React from 'react'
import { Link, useLocation } from 'react-router-dom'

// SVG Icon Components
const DashboardIcon = () => (
  <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="3" y="3" width="7" height="7"></rect>
    <rect x="14" y="3" width="7" height="7"></rect>
    <rect x="14" y="14" width="7" height="7"></rect>
    <rect x="3" y="14" width="7" height="7"></rect>
  </svg>
)

const AgentsIcon = () => (
  <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
    <circle cx="12" cy="7" r="4"></circle>
  </svg>
)

const TestRunsIcon = () => (
  <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10"></circle>
    <polygon points="10,8 16,12 10,16 10,8"></polygon>
  </svg>
)

const ResultsIcon = () => (
  <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="18" y1="20" x2="18" y2="10"></line>
    <line x1="12" y1="20" x2="12" y2="4"></line>
    <line x1="6" y1="20" x2="6" y2="14"></line>
  </svg>
)

const SettingsIcon = () => (
  <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="3"></circle>
    <path d="m12 1 1.27 1.27a9.96 9.96 0 0 1 2.46.37l1.84-.76.76 1.84a9.96 9.96 0 0 1 .37 2.46L20.17 7l.83 1-.83 1-1.47 1.27a9.96 9.96 0 0 1-.37 2.46l.76 1.84-1.84.76a9.96 9.96 0 0 1-2.46.37L13.73 17 12 17l-1.27-1.27a9.96 9.96 0 0 1-2.46-.37l-1.84.76-.76-1.84a9.96 9.96 0 0 1-.37-2.46L3.83 11 3 10l.83-1 1.47-1.27a9.96 9.96 0 0 1 .37-2.46L4.9 3.43l1.84-.76a9.96 9.96 0 0 1 2.46-.37L10.27 1 12 1z"></path>
  </svg>
)

const Layout = ({ children }) => {
  const location = useLocation()

  const isActive = (path) => {
    if (path === '/' && location.pathname === '/') return true
    if (path !== '/' && location.pathname.startsWith(path)) return true
    return false
  }

  return (
    <div className="container">
      <nav className="sidebar">
        <div className="sidebar-header">
          <h1>Armonite</h1>
        </div>
        <div className="sidebar-nav">
          <Link to="/" className={`nav-item ${isActive('/') ? 'active' : ''}`}>
            <DashboardIcon />
            Dashboard
          </Link>
          <Link to="/agents" className={`nav-item ${isActive('/agents') ? 'active' : ''}`}>
            <AgentsIcon />
            Agents
          </Link>
          <Link to="/test-runs" className={`nav-item ${isActive('/test-runs') ? 'active' : ''}`}>
            <TestRunsIcon />
            Test Runs
          </Link>
          <Link to="/results" className={`nav-item ${isActive('/results') ? 'active' : ''}`}>
            <ResultsIcon />
            Results
          </Link>
          <Link to="/settings" className={`nav-item ${isActive('/settings') ? 'active' : ''}`}>
            <SettingsIcon />
            Settings
          </Link>
        </div>
      </nav>

      <main className="main-content">
        {children}
      </main>
    </div>
  )
}

export default Layout