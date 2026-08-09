# ADR 0004: Use React + Vite with an Express API Layer for the Dashboard

**Status**: Accepted

**Date**: 2026-08-09

## Context

Sentinel needs a web dashboard for viewing and investigating log activity stored in Elasticsearch.

The dashboard should:

- Provide a component-based frontend for displaying log and anomaly data
- Avoid requiring the browser to query Elasticsearch directly
- Support a fast local development workflow
- Provide an API boundary between the frontend and Elasticsearch
- Allow Elasticsearch responses to be transformed or validated before reaching the frontend
- Support serving the built dashboard in production

## Decision

We will build the dashboard as a React application using Vite, with a Node.js/Express service providing the API layer between the frontend and Elasticsearch.

During development, Vite proxies `/api` requests to the Express server running on port 3001. Express provides API endpoints that query Elasticsearch and return responses to the frontend.

The browser therefore communicates with the Express API rather than querying Elasticsearch directly.

For production, Express serves the built React application from the `dist` directory and handles the API endpoints from the same server.

Next.js was considered unnecessary for the current dashboard requirements because the application does not currently require server-side rendering, static site generation, or framework-level routing. React + Vite with a small Express API layer provides the required functionality with less framework overhead.

## Consequences

### Positive

- **Elasticsearch isolation**: The browser does not directly query the Elasticsearch API
- **API boundary**: Express provides a dedicated layer for querying and shaping Elasticsearch responses
- **Fast development workflow**: Vite provides a development server with hot module replacement while Express handles API requests separately
- **Separation of concerns**: React handles the UI while Express handles backend API and Elasticsearch communication
- **Flexible API layer**: Request validation, response transformation, authentication, or caching can be added to Express without requiring direct Elasticsearch access from the frontend
- **Simple production serving**: Express can serve both the built React application and its API endpoints

### Negative

- **Additional server layer**: The dashboard requires an Express service in addition to the React/Vite frontend
- **Additional network hop**: API requests travel through Express before reaching Elasticsearch
- **Additional maintenance**: API endpoints and backend Elasticsearch integration must be maintained separately from the frontend
- **No built-in SSR/SSG**: If the dashboard later requires server-side rendering or static generation, additional architecture or a framework such as Next.js may need to be considered

### Neutral

- During development, Vite runs on port 5173 and proxies `/api` requests to Express on port 3001
- In production, Express serves the built React application and handles API requests from the same server
- Elasticsearch URL configuration is provided through the `ES_URL` environment variable, defaulting to `http://localhost:9200`
- The current `/api/logs` endpoint limits responses to 200 logs and sorts them by `timestamp_log` in descending order
- The current Elasticsearch connection does not configure authentication; the Express API isolates the browser from Elasticsearch's HTTP interface, but does not provide an authentication boundary for Elasticsearch itself

## Alternatives Considered

- **Next.js**: Provides an integrated React framework with routing and server-side rendering capabilities, but those features are not currently required by the dashboard. The additional framework conventions would add complexity relative to the current requirements
- **Direct browser-to-Elasticsearch queries**: Would remove the Express API layer, but would couple the frontend directly to Elasticsearch and expose the Elasticsearch API to the browser
- **React with another backend framework**: Could provide the same API boundary, but Express is sufficient for the dashboard's current API requirements
