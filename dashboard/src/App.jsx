
import React, { useEffect, useRef, useState } from 'react';
import axios from 'axios';
import { Satellite, Activity, Shield, Clock, Search, WifiOff } from 'lucide-react';
import Layout from './components/Layout';
import LogCard from './components/LogCard';
import DetailPanel from './components/DetailPanel';

const INITIAL_POLL_INTERVAL = 2000;
const MAX_POLL_INTERVAL = 30000;

function App() {
  const [logs, setLogs] = useState([]);
  const [selectedLog, setSelectedLog] = useState(null);
  const [stats, setStats] = useState({ total: 0, anomalies: 0, critical: 0 });
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [retryCount, setRetryCount] = useState(0);

  const retryCountRef = useRef(0);
  const timerRef = useRef(null);

  const scheduleNextFetch = (delay) => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
    }
    timerRef.current = setTimeout(() => {
      fetchLogs();
    }, delay);
  };

  const fetchLogs = async () => {
    try {
      const response = await axios.get('/api/logs', { params: { size: 50 } });
      const hits = response.data;
      setLogs(hits);

      setStats({
        total: hits.length,
        anomalies: hits.filter(l => l.is_anomaly).length,
        critical: hits.filter(l => (l.level || '').includes('error')).length
      });

      if (retryCountRef.current > 0) {
        retryCountRef.current = 0;
        setRetryCount(0);
        setIsReconnecting(false);
      }

      scheduleNextFetch(INITIAL_POLL_INTERVAL);
    } catch (error) {
      console.error("Connection Error:", error);
      retryCountRef.current += 1;
      const currentRetry = retryCountRef.current;
      setRetryCount(currentRetry);
      setIsReconnecting(true);

      const nextDelay = Math.min(
        INITIAL_POLL_INTERVAL * Math.pow(2, currentRetry),
        MAX_POLL_INTERVAL
      );
      scheduleNextFetch(nextDelay);
    }
  };

  useEffect(() => {
    fetchLogs();
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // --- Sidebar Content ---
  const SidebarContent = (
    <>
      <div style={{ marginBottom: 'var(--space-2xl)', color: 'var(--color-accent)' }}>
        <Satellite size={28} strokeWidth={1.5} />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        <div title="Live Feed" style={{ color: 'var(--color-white)', cursor: 'pointer' }}><Activity size={24} /></div>
        <div title="Security" style={{ color: 'var(--color-slate-600)', cursor: 'pointer' }}><Shield size={24} /></div>
        <div title="History" style={{ color: 'var(--color-slate-600)', cursor: 'pointer' }}><Clock size={24} /></div>
      </div>
    </>
  );

  // --- Header Content ---
  const HeaderContent = (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <Satellite size={20} className="text-info" />
        <h1 style={{ fontSize: '16px', fontWeight: 600, letterSpacing: '0.02em', color: 'var(--color-white)' }}>
          SENTINEL <span style={{ color: 'var(--color-slate-600)' }}>//</span> INCIDENT COMMAND
        </h1>
      </div>

      <div style={{ marginLeft: 'auto', display: 'flex', gap: '16px', alignItems: 'center' }}>
        {/* Status Indicators */}
        <div style={{ display: 'flex', gap: '12px', marginRight: '24px' }}>
          {isReconnecting && (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', lineHeight: '1' }}>
              <span style={{ fontSize: '10px', color: 'var(--color-warning)', textTransform: 'uppercase', display: 'flex', alignItems: 'center', gap: '3px' }}>
                <WifiOff size={10} /> Reconnecting...
              </span>
              <span style={{ fontSize: '12px', fontWeight: 'bold', color: 'var(--color-warning)' }} data-testid="reconnecting-status">
                Retry in {Math.min(2 * Math.pow(2, retryCount), 30)}s
              </span>
            </div>
          )}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', lineHeight: '1' }}>
            <span style={{ fontSize: '10px', color: 'var(--color-slate-400)', textTransform: 'uppercase' }}>Anomalies</span>
            <span style={{ fontSize: '14px', fontWeight: 'bold', color: stats.anomalies > 0 ? 'var(--color-anomaly)' : 'var(--color-slate-200)' }}>{stats.anomalies}</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', lineHeight: '1' }}>
            <span style={{ fontSize: '10px', color: 'var(--color-slate-400)', textTransform: 'uppercase' }}>Critical</span>
            <span style={{ fontSize: '14px', fontWeight: 'bold', color: stats.critical > 0 ? 'var(--color-danger)' : 'var(--color-slate-200)' }}>{stats.critical}</span>
          </div>
        </div>

        {/* Search Bar Mockup */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          backgroundColor: 'var(--color-slate-900)',
          border: '1px solid var(--color-slate-700)',
          borderRadius: '4px',
          padding: '4px 8px',
          color: 'var(--color-slate-400)'
        }}>
          <Search size={14} style={{ marginRight: '8px' }} />
          <span style={{ fontSize: '12px' }}>Search logs...</span>
        </div>
      </div>
    </>
  );

  return (
    <Layout
      sidebar={SidebarContent}
      header={HeaderContent}
      rightPanel={<DetailPanel log={selectedLog} onClose={() => setSelectedLog(null)} />}
    >
      {/* Feed Content */}
      <div className="fade-in">
        {logs.map((log, index) => (
          <LogCard
            key={log._id || index}
            log={log}
            isSelected={selectedLog === log}
            onClick={() => setSelectedLog(log)}
          />
        ))}
        {logs.length === 0 && (
          <div style={{ padding: '40px', textAlign: 'center', color: 'var(--color-slate-600)' }}>
            {isReconnecting ? 'Connection lost. Reconnecting to Sentinel backend...' : 'Waiting for incoming transmission...'}
          </div>
        )}
      </div>
    </Layout>
  );
}

export default App;