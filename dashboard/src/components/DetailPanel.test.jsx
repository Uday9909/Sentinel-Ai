import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import DetailPanel from './DetailPanel.jsx';

describe('DetailPanel', () => {
  it('shows empty state when no log is selected', () => {
    render(<DetailPanel log={null} onClose={() => {}} />);
    expect(screen.getByText('Select a log to view details')).toBeInTheDocument();
  });

  it('renders log message when selected', () => {
    const log = {
      message: 'Connection timeout',
      service: 'api',
      level: 'error',
      timestamp_log: 1716912330,
      is_anomaly: false,
    };
    render(<DetailPanel log={log} onClose={() => {}} />);
    expect(screen.getByText('Connection timeout')).toBeInTheDocument();
  });

  it('shows AI analysis section when is_anomaly is true', () => {
    const log = {
      message: 'Critical failure',
      service: 'api',
      level: 'error',
      timestamp_log: 1716912330,
      is_anomaly: true,
      ai_explanation: 'Database pool exhausted. Fix: restart.',
    };
    render(<DetailPanel log={log} onClose={() => {}} />);
    expect(screen.getByText('AI Root Cause Analysis')).toBeInTheDocument();
    expect(screen.getByText('Database pool exhausted. Fix: restart.')).toBeInTheDocument();
  });

  it('does not show AI section when is_anomaly is false', () => {
    const log = {
      message: 'Ok',
      service: 'api',
      level: 'info',
      timestamp_log: 1716912330,
      is_anomaly: false,
    };
    render(<DetailPanel log={log} onClose={() => {}} />);
    expect(screen.queryByText('AI Root Cause Analysis')).toBeNull();
  });

  it('uses timestamp_log for display', () => {
    const log = {
      message: 'test',
      service: 'api',
      level: 'info',
      timestamp_log: 1716912330,
      is_anomaly: false,
    };
    render(<DetailPanel log={log} onClose={() => {}} />);
    // Timestamp is rendered in locale format — just confirm it's present
    expect(screen.getByText(/2025|28|18.*05/)).toBeInTheDocument();
  });

  it('falls back to timestamp when timestamp_log is missing', () => {
    const log = {
      message: 'test',
      service: 'api',
      level: 'info',
      timestamp: 1716912330,
      is_anomaly: false,
    };
    render(<DetailPanel log={log} onClose={() => {}} />);
    expect(screen.getByText(/2025|28/)).toBeInTheDocument();
  });
});
