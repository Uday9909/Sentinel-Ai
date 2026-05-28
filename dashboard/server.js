import express from 'express';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const ES_URL = process.env.ES_URL || 'http://localhost:9200';
const PORT = process.env.PORT || 3001;

const app = express();
app.use(express.json());

// -----------------------------------------------------------------------
// API endpoints
// -----------------------------------------------------------------------

app.get('/api/health', (_req, res) => {
  res.json({ status: 'ok' });
});

app.get('/api/logs', async (req, res) => {
  try {
    const size = Math.min(parseInt(req.query.size) || 50, 200);
    const esRes = await fetch(`${ES_URL}/logs-index/_search`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        size,
        sort: [{ timestamp_log: { order: 'desc', unmapped_type: 'boolean' } }],
      }),
    });

    if (!esRes.ok) {
      throw new Error(`Elasticsearch returned ${esRes.status}`);
    }

    const data = await esRes.json();
    const logs = (data.hits?.hits || []).map(hit => ({
      ...hit._source,
      _id: hit._id,
    }));
    res.json(logs);
  } catch (err) {
    console.error('ES query failed:', err.message);
    res.status(502).json({ error: 'Failed to fetch logs' });
  }
});

// -----------------------------------------------------------------------
// Static file serving (production — built React app)
// -----------------------------------------------------------------------

const distPath = join(__dirname, 'dist');
app.use(express.static(distPath));

// SPA fallback: serve index.html for all non-API routes
app.get('*', (req, res, next) => {
  if (req.path.startsWith('/api/')) return next();
  res.sendFile(join(distPath, 'index.html'), (err) => {
    if (err) next();
  });
});

// -----------------------------------------------------------------------
// Start
// -----------------------------------------------------------------------

const isMain = process.argv[1] === __filename;
if (isMain) {
  app.listen(PORT, () => {
    console.log(`Sentinel dashboard server running on http://localhost:${PORT}`);
    console.log(`Proxying ES requests to ${ES_URL}`);
  });
}

export { app, ES_URL };
