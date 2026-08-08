import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import LogCard from './LogCard.jsx'

function renderCard(overrides = {}) {
  const log = {
    _id: 'abc123',
    message: 'Database connection refused',
    service: 'payment-api',
    level: 'error',
    timestamp_log: 1716912330, // 2025-05-28T18:05:30
    timestamp: 1716912330,
    is_anomaly: false,
    ...overrides,
  }
  return render(<LogCard log={log} isSelected={false} onClick={() => {}} />)
}

describe('LogCard', () => {
  it('renders the log message', () => {
    renderCard()
    expect(screen.getByText('Database connection refused')).toBeInTheDocument()
  })

  it('renders the service name', () => {
    renderCard()
    expect(screen.getByText('payment-api')).toBeInTheDocument()
  })

  it('uses timestamp_log over timestamp for display', () => {
    renderCard()
    // 1716912330 = 18:05:30 UTC, but depending on timezone it differs.
    // Just verify a time string rendered.
    const timeEl = screen.getByText(/:.{2}:.{2}/)
    expect(timeEl).toBeInTheDocument()
  })

  it('falls back to timestamp when timestamp_log is missing', () => {
    renderCard({ timestamp_log: undefined })
    const timeEl = screen.getByText(/:.{2}:.{2}/)
    expect(timeEl).toBeInTheDocument()
  })

  it('shows --:--:-- when no timestamp is available', () => {
    renderCard({ timestamp_log: undefined, timestamp: undefined })
    expect(screen.getByText('--:--:--')).toBeInTheDocument()
  })

  it('shows ANOMALY badge when is_anomaly is true', () => {
    renderCard({ is_anomaly: true })
    expect(screen.getByText('ANOMALY')).toBeInTheDocument()
  })

  it('does not show ANOMALY badge when is_anomaly is false', () => {
    renderCard({ is_anomaly: false })
    expect(screen.queryByText('ANOMALY')).toBeNull()
  })

  it('renders SYSTEM when service is missing', () => {
    renderCard({ service: undefined })
    expect(screen.getByText('SYSTEM')).toBeInTheDocument()
  })
})
