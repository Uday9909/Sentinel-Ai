// @vitest-environment node
import { describe, it, expect, afterAll, vi } from 'vitest'
import request from 'supertest'

// Must set before importing the app so the ES_URL env is read at module init.
process.env.ES_URL = 'http://elasticsearch-test:9200'

const { app } = await import('./server.js')

afterAll(() => {
  vi.unstubAllGlobals()
})

function mockEsResponse(data, status = 200) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      json: () => Promise.resolve(data),
    }),
  )
}

describe('GET /api/health', () => {
  it('returns ok', async () => {
    const res = await request(app).get('/api/health')
    expect(res.status).toBe(200)
    expect(res.body).toEqual({ status: 'ok' })
  })
})

describe('GET /api/logs', () => {
  it('returns logs from ES', async () => {
    mockEsResponse({
      hits: {
        hits: [
          {
            _id: '1',
            _source: { message: 'Error', service: 'api', level: 'error' },
          },
          {
            _id: '2',
            _source: { message: 'Ok', service: 'api', level: 'info' },
          },
        ],
      },
    })

    const res = await request(app).get('/api/logs')
    expect(res.status).toBe(200)
    expect(res.body).toHaveLength(2)
    expect(res.body[0]).toMatchObject({ _id: '1', message: 'Error' })
    expect(res.body[1]).toMatchObject({ _id: '2', message: 'Ok' })
  })

  it('caps size at 200', async () => {
    mockEsResponse({ hits: { hits: [] } })
    const res = await request(app).get('/api/logs?size=999')
    expect(res.status).toBe(200)
  })

  it('returns 502 on ES error status', async () => {
    mockEsResponse({}, 503)
    const res = await request(app).get('/api/logs')
    expect(res.status).toBe(502)
    expect(res.body.error).toContain('Failed to fetch logs')
  })

  it('returns 502 on network error', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockRejectedValue(new Error('connection refused')),
    )
    const res = await request(app).get('/api/logs')
    expect(res.status).toBe(502)
  })

  it('handles empty ES response', async () => {
    mockEsResponse({})
    const res = await request(app).get('/api/logs')
    expect(res.status).toBe(200)
    expect(res.body).toEqual([])
  })
})
