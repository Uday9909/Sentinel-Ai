import React from 'react';
import { X, Server, Clock, AlertOctagon, BrainCircuit } from 'lucide-react';

const DetailRow = ({ label, value, mono = false }) => (
    <div style={{ marginBottom: '12px' }}>
        <div style={{ fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-slate-600)', letterSpacing: '0.05em' }}>
            {label}
        </div>
        <div style={{
            fontSize: '13px',
            color: 'var(--color-slate-200)',
            fontFamily: mono ? 'var(--font-mono)' : 'inherit',
            wordBreak: 'break-all'
        }}>
            {value}
        </div>
    </div>
);

const DetailPanel = ({ log, onClose }) => {
    if (!log) {
        return (
            <div style={{
                height: '100%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--color-slate-600)',
                fontSize: '13px'
            }}>
                <div style={{ textAlign: 'center' }}>
                    <Server size={48} style={{ opacity: 0.2, marginBottom: '16px' }} />
                    <p>Select a log to view details</p>
                </div>
            </div>
        );
    }

    const isAnomaly = log.is_anomaly;

    return (
        <div className="fade-in" style={{ padding: 'var(--space-xl)' }}>
            {/* Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', marginBottom: 'var(--space-xl)' }}>
                <h2 style={{ fontSize: '18px', fontWeight: 600, color: 'var(--color-white)' }}>
                    Incident Details
                </h2>
            </div>

            {/* AI Analysis Section - Prominent if Anomaly */}
            {isAnomaly && (
                <div style={{
                    backgroundColor: 'rgba(2, 6, 23, 0.5)',
                    border: '1px solid var(--color-info)',
                    borderRadius: '8px',
                    padding: 'var(--space-md)',
                    marginBottom: 'var(--space-xl)',
                    boxShadow: '0 0 20px rgba(59, 130, 246, 0.1)'
                }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px', color: 'var(--color-info)' }}>
                        <BrainCircuit size={16} />
                        <span style={{ fontSize: '12px', fontWeight: 'bold', textTransform: 'uppercase' }}>AI Root Cause Analysis</span>
                    </div>

                    <p style={{ fontSize: '13px', lineHeight: '1.6', color: 'var(--color-slate-200)' }}>
                        {log.ai_explanation || "Analyzing system behavior..."}
                    </p>

                    {/* Actionable Suggestions (Mockup logic based on text) */}
                    <div style={{ marginTop: '12px', paddingTop: '12px', borderTop: '1px solid var(--color-slate-800)' }}>
                        <p style={{ fontSize: '11px', color: 'var(--color-slate-400)' }}>Suggested Action: Check database connectivity and restart connection pool.</p>
                    </div>
                </div>
            )}

            {/* Log Metadata */}
            <DetailRow label="Message" value={log.message} />
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <DetailRow label="Service" value={log.service || 'Unknown'} />
                <DetailRow label="Level" value={log.level?.toUpperCase()} />
            </div>

            <DetailRow
                label="Timestamp"
                value={new Date(log.timestamp * 1000).toLocaleString()}
                mono
            />

            <DetailRow
                label="Cluster ID"
                value={log.cluster_id || 'Unclassified'}
                mono
            />

            {/* Raw JSON View */}
            <div style={{ marginTop: 'var(--space-2xl)' }}>
                <div style={{ fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-slate-600)', marginBottom: '8px' }}>
                    Raw JSON Payload
                </div>
                <pre style={{
                    backgroundColor: 'var(--color-slate-950)',
                    padding: 'var(--space-md)',
                    borderRadius: '4px',
                    fontSize: '11px',
                    fontFamily: 'var(--font-mono)',
                    color: 'var(--color-slate-400)',
                    overflowX: 'auto',
                    border: '1px solid var(--color-slate-800)'
                }}>
                    {JSON.stringify(log, null, 2)}
                </pre>
            </div>
        </div>
    );
};

export default DetailPanel;
