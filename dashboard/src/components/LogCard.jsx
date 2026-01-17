import React from 'react';
import { AlertCircle, CheckCircle, Info, AlertTriangle, Cpu, Terminal } from 'lucide-react';

const getSeverityConfig = (level) => {
    const l = (level || '').toLowerCase();

    if (['fatal', 'critical', 'error'].some(s => l.includes(s))) {
        return { color: 'var(--color-danger)', icon: AlertTriangle, bg: 'rgba(239, 68, 68, 0.1)' };
    }
    if (['warn', 'warning'].some(s => l.includes(s))) {
        return { color: 'var(--color-warning)', icon: AlertCircle, bg: 'rgba(249, 115, 22, 0.1)' };
    }
    if (['success'].some(s => l.includes(s))) {
        return { color: 'var(--color-success)', icon: CheckCircle, bg: 'rgba(16, 185, 129, 0.1)' };
    }
    return { color: 'var(--color-info)', icon: Info, bg: 'rgba(59, 130, 246, 0.1)' };
};

const LogCard = ({ log, isSelected, onClick }) => {
    const config = getSeverityConfig(log.level);
    const Icon = config.icon;
    const isAnomaly = log.is_anomaly;

    // Format timestamp (just time for the card, e.g., 14:05:22)
    const timeStr = log.timestamp
        ? new Date(log.timestamp * 1000).toLocaleTimeString([], { hour12: false })
        : '--:--:--';

    return (
        <div
            onClick={onClick}
            className="fade-in"
            style={{
                padding: 'var(--space-md) var(--space-lg)',
                borderBottom: '1px solid var(--color-slate-800)',
                borderLeft: `4px solid ${isAnomaly ? 'var(--color-anomaly)' : config.color}`,
                backgroundColor: isSelected ? 'var(--color-slate-800)' : 'transparent',
                cursor: 'pointer',
                transition: 'background-color 0.1s ease',
                position: 'relative' // for anomaly pulsing
            }}
            onMouseEnter={(e) => e.currentTarget.style.backgroundColor = 'var(--color-slate-800)'}
            onMouseLeave={(e) => !isSelected && (e.currentTarget.style.backgroundColor = 'transparent')}
        >
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: '4px', gap: '8px' }}>
                {/* Severity Icon */}
                <Icon size={14} color={config.color} />

                {/* Service Name Badge */}
                <span style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    color: 'var(--color-slate-400)',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px'
                }}>
                    <Cpu size={10} />
                    {log.service || 'SYSTEM'}
                </span>

                {/* Timestamp */}
                <span style={{
                    marginLeft: 'auto',
                    fontSize: '11px',
                    fontFamily: 'var(--font-mono)',
                    color: 'var(--color-slate-600)'
                }}>
                    {timeStr}
                </span>
            </div>

            {/* Message */}
            <div style={{
                color: 'var(--color-slate-200)',
                fontSize: '13px',
                fontWeight: 500,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis'
            }}>
                {log.message}
            </div>

            {/* Anomaly Badge (if applicable) */}
            {isAnomaly && (
                <div style={{
                    position: 'absolute',
                    right: '16px',
                    top: '36px',
                    color: 'var(--color-white)',
                    backgroundColor: 'var(--color-anomaly)',
                    fontSize: '9px',
                    fontWeight: 'bold',
                    padding: '2px 6px',
                    borderRadius: '4px',
                    boxShadow: '0 0 10px var(--color-anomaly)'
                }}>
                    ANOMALY
                </div>
            )}
        </div>
    );
};

export default LogCard;
