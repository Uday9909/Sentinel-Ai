import React from 'react';

const Layout = ({ header, sidebar, children, rightPanel }) => {
    return (
        <div style={{
            display: 'grid',
            gridTemplateColumns: 'var(--sidebar-width) 1fr var(--detail-panel-width)',
            gridTemplateRows: 'var(--header-height) 1fr',
            height: '100vh',
            backgroundColor: 'var(--color-slate-950)'
        }}>
            {/* Sidebar - Spans full height */}
            <aside style={{
                gridRow: '1 / -1',
                gridColumn: '1 / 2',
                borderRight: '1px solid var(--color-slate-700)',
                backgroundColor: 'var(--color-slate-900)',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                padding: 'var(--space-md) 0',
                zIndex: 20
            }}>
                {sidebar}
            </aside>

            {/* Header - Top of main content area */}
            <header style={{
                gridRow: '1 / 2',
                gridColumn: '2 / 4',
                borderBottom: '1px solid var(--color-slate-700)',
                backgroundColor: 'rgba(15, 23, 42, 0.8)', /* slate-900 with opacity */
                backdropFilter: 'blur(8px)',
                display: 'flex',
                alignItems: 'center',
                padding: '0 var(--space-xl)',
                zIndex: 10
            }}>
                {header}
            </header>

            {/* Main Feed Area */}
            <main style={{
                gridRow: '2 / -1',
                gridColumn: '2 / 3',
                overflowY: 'auto',
                padding: '0',
                position: 'relative'
            }}>
                {children}
            </main>

            {/* Right Detail Panel */}
            <aside style={{
                gridRow: '2 / -1',
                gridColumn: '3 / 4',
                borderLeft: '1px solid var(--color-slate-700)',
                backgroundColor: 'var(--color-slate-900)',
                overflowY: 'auto'
            }}>
                {rightPanel}
            </aside>
        </div>
    );
};

export default Layout;
