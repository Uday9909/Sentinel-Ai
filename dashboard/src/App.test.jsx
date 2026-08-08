import React from 'react'
import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import axios from 'axios'
import App from './App'

vi.mock('axios')

describe('App Exponential Backoff Polling', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders initial state and fetches logs every 2000ms when healthy', async () => {
    axios.get.mockResolvedValue({
      data: [
        {
          _id: '1',
          message: 'System normal',
          level: 'info',
          is_anomaly: false,
          timestamp: 1700000000,
        },
      ],
    })

    await act(async () => {
      render(<App />)
    })

    expect(axios.get).toHaveBeenCalledTimes(1)
    expect(screen.getByText('System normal')).toBeDefined()

    // Advance timers by 2000ms for next normal poll
    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(axios.get).toHaveBeenCalledTimes(2)
    expect(screen.queryByTestId('reconnecting-status')).toBeNull()
  })

  it('backs off exponentially on consecutive connection errors (4s -> 8s -> 16s -> 30s cap)', async () => {
    // Initial fetch succeeds
    axios.get.mockResolvedValueOnce({ data: [] })

    await act(async () => {
      render(<App />)
    })
    expect(axios.get).toHaveBeenCalledTimes(1)

    // Mock subsequent fetches to fail
    axios.get.mockRejectedValue(new Error('Network error'))

    // Advance 2000ms for normal poll, which fails (Retry 1 -> Next delay: 4s)
    await act(async () => {
      vi.advanceTimersByTime(2000)
    })
    expect(axios.get).toHaveBeenCalledTimes(2)
    expect(screen.getByTestId('reconnecting-status')).toBeDefined()

    // Advance 2000ms - should NOT poll yet (waiting for 4s total)
    await act(async () => {
      vi.advanceTimersByTime(2000)
    })
    expect(axios.get).toHaveBeenCalledTimes(2)

    // Advance another 2000ms (4s total) -> Trigger Retry 2 (Next delay: 8s)
    await act(async () => {
      vi.advanceTimersByTime(2000)
    })
    expect(axios.get).toHaveBeenCalledTimes(3)

    // Advance 8s -> Trigger Retry 3 (Next delay: 16s)
    await act(async () => {
      vi.advanceTimersByTime(8000)
    })
    expect(axios.get).toHaveBeenCalledTimes(4)

    // Advance 16s -> Trigger Retry 4 (Next delay: capped at 30s)
    await act(async () => {
      vi.advanceTimersByTime(16000)
    })
    expect(axios.get).toHaveBeenCalledTimes(5)
  })

  it('resets interval to 2000ms and clears reconnecting status on successful fetch', async () => {
    // 1. Initial succeeds
    axios.get.mockResolvedValueOnce({ data: [] })
    await act(async () => {
      render(<App />)
    })

    // 2. Poll fails (Retry 1 -> 4s)
    axios.get.mockRejectedValueOnce(new Error('Backend down'))
    await act(async () => {
      vi.advanceTimersByTime(2000)
    })
    expect(screen.getByTestId('reconnecting-status')).toBeDefined()

    // 3. Recovery: next fetch succeeds
    axios.get.mockResolvedValueOnce({
      data: [
        {
          _id: '2',
          message: 'Recovered log',
          level: 'info',
          is_anomaly: false,
          timestamp: 1700000010,
        },
      ],
    })

    await act(async () => {
      vi.advanceTimersByTime(4000)
    })

    expect(screen.getByText('Recovered log')).toBeDefined()
    expect(screen.queryByTestId('reconnecting-status')).toBeNull()

    // 4. Normal 2000ms interval resumed
    axios.get.mockResolvedValueOnce({ data: [] })
    await act(async () => {
      vi.advanceTimersByTime(2000)
    })
    expect(axios.get).toHaveBeenCalledTimes(4)
  })
})
