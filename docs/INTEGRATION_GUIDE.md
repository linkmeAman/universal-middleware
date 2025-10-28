# Universal Middleware Integration Guide

## Table of Contents
1. [Overview](#overview)
2. [Use Cases](#use-cases)
3. [Integration Methods](#integration-methods)
4. [Quick Start](#quick-start)
5. [Authentication](#authentication)
6. [API Integration](#api-integration)
7. [Real-time Integration](#real-time-integration)
8. [Error Handling](#error-handling)
9. [Best Practices](#best-practices)
10. [Examples](#examples)

## Overview

Universal Middleware is a high-performance, cache-first, event-driven middleware platform that can be integrated with your applications in several ways:

- REST API Gateway
- WebSocket Real-time Updates
- Event-driven Command Processing
- Cache Layer
- Authentication/Authorization Layer

## Use Cases

1. **API Gateway Layer**
   - Rate limiting and throttling
   - Authentication and authorization
   - Request validation
   - Response caching
   - Load balancing

2. **Caching Layer**
   - High-performance data access
   - Reduced database load
   - Cache invalidation handling
   - Distributed caching

3. **Real-time Updates**
   - WebSocket-based notifications
   - Live data updates
   - Chat applications
   - Real-time dashboards

4. **Command Processing**
   - Async operation handling
   - Guaranteed message delivery
   - Idempotent operations
   - Event sourcing

## Integration Methods

### 1. REST API Gateway (Port 8080)
```javascript
const API_BASE = 'http://your-middleware:8080/v1';

class MiddlewareClient {
  constructor(config) {
    this.config = config;
    this.token = config.token;
  }

  async get(endpoint) {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Accept': 'application/json'
      }
    });
    return response.json();
  }

  async post(endpoint, data) {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
        'Idempotency-Key': generateUUID()
      },
      body: JSON.stringify(data)
    });
    return response.json();
  }
}
```

### 2. WebSocket Integration (Port 8085)
```javascript
class RealtimeClient {
  constructor(config) {
    this.ws = new WebSocket(`ws://your-middleware:8085/ws?token=${config.token}`);
    this.subscriptions = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = config.maxReconnectAttempts || 5;
    this.setupReconnection();
    
    this.ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      this.handleMessage(data);
    };
  }

  subscribe(topic, callback) {
    this.ws.send(JSON.stringify({
      type: 'join_room',
      room: topic
    }));
    this.subscriptions.set(topic, callback);
  }

  setupReconnection() {
    this.ws.onclose = () => {
      if (this.reconnectAttempts < this.maxReconnectAttempts) {
        setTimeout(() => {
          this.reconnectAttempts++;
          this.ws = new WebSocket(`ws://your-middleware:8085/ws?token=${this.config.token}`);
          this.setupReconnection();
          
          // Resubscribe to previous topics
          for (let [topic] of this.subscriptions) {
            this.subscribe(topic);
          }
        }, Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000));
      }
    };

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
    };
  }

  handleMessage(data) {
    const callback = this.subscriptions.get(data.topic);
    if (callback) callback(data);
  }

  disconnect() {
    this.maxReconnectAttempts = 0; // Prevent reconnection
    this.ws.close();
  }
}
```

## Quick Start

1. **Install Dependencies**
```bash
# If using npm
npm install universal-middleware-client

# If using yarn
yarn add universal-middleware-client
```

2. **Basic Setup**
```javascript
import { MiddlewareClient } from 'universal-middleware-client';

const client = new MiddlewareClient({
  baseUrl: 'http://your-middleware:8080',
  wsUrl: 'ws://your-middleware:8085',
  token: 'your-auth-token'
});
```

3. **Make API Calls**
```javascript
// Read operation (cached)
const data = await client.get('/entities/123');

// Write operation (command)
const result = await client.post('/entities', {
  name: 'Test Entity',
  value: 42
});
```

## Authentication

### OAuth2/OIDC Setup
```javascript
const authConfig = {
  authority: 'https://your-auth-server',
  client_id: 'your-client-id',
  redirect_uri: 'http://your-app/callback',
  response_type: 'code',
  scope: 'openid profile api'
};

// Initialize authentication
const auth = new AuthClient(authConfig);
const token = await auth.getToken();

// Use token with middleware
const client = new MiddlewareClient({
  ...config,
  token: token
});
```

## API Integration

### Read Operations
```javascript
// Get entity by ID (cached)
const entity = await client.get('/entities/123');

// List entities with pagination
const list = await client.get('/entities?page=1&limit=10');

// Search entities
const searchResults = await client.get('/entities?q=searchterm');
```

### Write Operations
```javascript
// Create entity
const created = await client.post('/entities', {
  name: 'New Entity',
  data: { ... }
});

// Update entity
const updated = await client.patch('/entities/123', {
  name: 'Updated Name'
});

// Delete entity
await client.delete('/entities/123');
```

## Real-time Integration

### WebSocket Setup
```javascript
const realtime = new RealtimeClient({
  url: 'ws://your-middleware:8085',
  token: 'your-auth-token'
});

// Subscribe to entity updates
realtime.subscribe('entity.123', (update) => {
  console.log('Entity updated:', update);
});

// Subscribe to all entities
realtime.subscribe('entity.*', (update) => {
  console.log('Any entity updated:', update);
});
```

## Error Handling

```javascript
try {
  const result = await client.post('/entities', data);
} catch (error) {
  switch (error.code) {
    case 'INVALID_REQUEST':
      // Handle validation errors
      break;
    case 'UNAUTHORIZED':
      // Handle authentication errors
      break;
    case 'RATE_LIMITED':
      // Handle rate limiting
      break;
    default:
      // Handle unknown errors
      break;
  }
}
```

## Best Practices

1. **Always Use Idempotency Keys**
```javascript
const result = await client.post('/entities', data, {
  idempotencyKey: generateUUID()
});
```

2. **Implement Retry Logic**
```javascript
const result = await retry(async () => {
  return await client.post('/entities', data);
}, {
  retries: 3,
  backoff: 'exponential'
});
```

3. **Cache Handling**
```javascript
// Use ETags for caching
const { data, etag } = await client.get('/entities/123');

// Subsequent requests
const updated = await client.get('/entities/123', {
  headers: { 'If-None-Match': etag }
});
```

## Examples

### 1. Simple CRUD Application
```javascript
class EntityManager {
  constructor(client) {
    this.client = client;
  }

  async getEntity(id) {
    return await this.client.get(`/entities/${id}`);
  }

  async createEntity(data) {
    return await this.client.post('/entities', data);
  }

  async updateEntity(id, data) {
    return await this.client.patch(`/entities/${id}`, data);
  }

  async deleteEntity(id) {
    return await this.client.delete(`/entities/${id}`);
  }
}
```

### 2. Real-time Dashboard
```javascript
class DashboardApp {
  constructor(client, realtime) {
    this.client = client;
    this.realtime = realtime;
    this.setupRealtime();
  }

  async setupRealtime() {
    // Subscribe to updates
    this.realtime.subscribe('metrics.*', (update) => {
      this.updateDashboard(update);
    });

    // Initial data load
    const metrics = await this.client.get('/metrics');
    this.displayMetrics(metrics);
  }

  updateDashboard(update) {
    // Update UI with real-time data
  }

  displayMetrics(metrics) {
    // Display initial metrics
  }
}
```

### 3. Command Processing
```javascript
class CommandProcessor {
  constructor(client) {
    this.client = client;
  }

  async submitCommand(command) {
    // Submit command
    const { command_id } = await this.client.post('/commands', command);
    
    // Poll for completion
    return this.pollCommandStatus(command_id);
  }

  async pollCommandStatus(commandId) {
    while (true) {
      const status = await this.client.get(`/commands/${commandId}/status`);
      if (status.state === 'completed') return status.result;
      if (status.state === 'failed') throw new Error(status.error);
      await sleep(1000);
    }
  }
}
```

## Error Handling

### WebSocket Errors
```javascript
class WebSocketError extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
  }
}

class RealtimeClient {
  handleError(error) {
    switch (error.code) {
      case 'RATE_LIMITED':
        // Wait before reconnecting
        setTimeout(() => this.reconnect(), 5000);
        break;
      case 'INVALID_TOKEN':
        // Refresh token and reconnect
        this.refreshToken().then(() => this.reconnect());
        break;
      case 'SUBSCRIPTION_ERROR':
        // Remove failed subscription
        this.subscriptions.delete(error.topic);
        break;
      default:
        // Log unhandled error
        console.error('WebSocket error:', error);
    }
  }
}
```

## Health Monitoring

```javascript
class HealthMonitor {
  constructor(client) {
    this.client = client;
  }

  async checkHealth() {
    try {
      const health = await this.client.get('/health');
      return {
        status: health.status,
        services: health.services,
        timestamp: new Date()
      };
    } catch (error) {
      return {
        status: 'unhealthy',
        error: error.message,
        timestamp: new Date()
      };
    }
  }

  startMonitoring(interval = 60000) {
    setInterval(() => this.checkHealth(), interval);
  }
}
```

For more detailed information and advanced usage, please refer to the API documentation and RFC specifications.