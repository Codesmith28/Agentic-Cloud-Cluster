# Web Interface - Future Implementation

This document outlines the planned features and API endpoints for the CloudAI web interface.

## Overview

The web interface will provide a user-friendly dashboard for:
- Task submission and management
- Live task monitoring with log streaming
- Worker status visualization
- Resource utilization metrics
- User authentication and authorization

## User Features

### 1. Authentication & Authorization
- **Login/Logout**: User authentication with JWT tokens
- **User Dashboard**: Personalized view of user's tasks and resources
- **Role-Based Access**: Different permissions for users vs admins

### 2. Task Management

#### Task Submission
- **Form-based submission**: Web form to submit new tasks
- **Fields**:
  - Docker image (text input with validation)
  - Command to execute (textarea)
  - Resource requirements (CPU, Memory, Storage, GPU)
  - Task name/description (optional)
- **Validation**: Client-side and server-side validation of inputs
- **Submission feedback**: Real-time status of task acceptance

#### Task List View
- **My Tasks**: List of all tasks submitted by the user
- **Columns**: Task ID, Name, Status, Docker Image, Worker, Created, Runtime
- **Filtering**: By status (pending, running, completed, failed)
- **Sorting**: By creation time, status, duration
- **Search**: Search by task ID or name

#### Task Detail View
- **Task Information**: Full details about a specific task
  - Task ID, User ID, Status
  - Docker image and command
  - Resource requirements and allocations
  - Timestamps (created, started, completed)
  - Assigned worker information
- **Actions**:
  - Cancel task (if running)
  - Retry task (if failed)
  - Delete task
  - Download logs

### 3. Live Task Monitoring

#### Real-time Log Streaming
- **WebSocket Connection**: Live log streaming from worker
- **Auto-scroll**: Automatically scroll to latest logs
- **Pause/Resume**: Pause auto-scroll to review previous logs
- **Search in logs**: Real-time search within log output
- **Log filters**: Filter by severity (stdout/stderr)
- **Color coding**: Syntax highlighting for logs
- **Download**: Export logs to file

#### Live Metrics Display
- **Resource Usage**: Real-time CPU, memory, GPU usage graphs
- **Network I/O**: Network traffic metrics
- **Disk I/O**: Storage read/write metrics
- **Progress indicators**: Visual progress bars for long-running tasks

#### Multi-task Monitoring
- **Dashboard view**: Monitor multiple running tasks simultaneously
- **Grid layout**: Tiled view of multiple task logs
- **Notifications**: Browser notifications for task completion/failure

### 4. Worker Visualization

#### Worker Status Dashboard
- **Worker list**: All registered workers with status indicators
- **Health status**: Active, inactive, overloaded
- **Resource utilization**: Bar charts for CPU, memory, storage, GPU
- **Task distribution**: Number of tasks per worker
- **Network topology**: Visual map of worker connections

#### Worker Details
- **Worker information**: Hostname, IP, specs
- **Capacity metrics**: Total vs available resources
- **Running tasks**: List of tasks currently on this worker
- **Historical metrics**: Resource usage over time (graphs)

### 5. Admin Features

#### User Management
- **User list**: All registered users
- **User details**: Task history, resource usage statistics
- **Create/Edit/Delete users**: User CRUD operations
- **Set quotas**: Resource limits per user

#### System Overview
- **Cluster statistics**: Total workers, tasks, resource utilization
- **System health**: Overall cluster status
- **Task queue**: Pending tasks waiting for assignment
- **Logs**: System-wide logs and events

## API Endpoints

### Authentication
```
POST   /api/auth/login          - User login
POST   /api/auth/logout         - User logout
POST   /api/auth/register       - User registration
GET    /api/auth/profile        - Get current user profile
```

### Tasks
```
POST   /api/tasks               - Submit new task
GET    /api/tasks               - List user's tasks (with filters)
GET    /api/tasks/:id           - Get task details
DELETE /api/tasks/:id           - Cancel/delete task
POST   /api/tasks/:id/retry     - Retry failed task
GET    /api/tasks/:id/logs      - Get stored logs for completed task
```

### Live Monitoring
```
WS     /api/tasks/:id/stream    - WebSocket for live log streaming
GET    /api/tasks/:id/metrics   - Get real-time metrics
WS     /api/tasks/:id/metrics/stream - WebSocket for live metrics
```

### Workers (Read-only for users, CRUD for admins)
```
GET    /api/workers             - List all workers
GET    /api/workers/:id         - Get worker details
GET    /api/workers/:id/metrics - Get worker resource metrics
GET    /api/workers/:id/tasks   - Get tasks assigned to worker
```

### Admin Endpoints
```
POST   /api/admin/workers       - Register new worker
PUT    /api/admin/workers/:id   - Update worker configuration
DELETE /api/admin/workers/:id   - Deregister worker

GET    /api/admin/users         - List all users
POST   /api/admin/users         - Create new user
PUT    /api/admin/users/:id     - Update user
DELETE /api/admin/users/:id     - Delete user

GET    /api/admin/stats         - Get cluster statistics
GET    /api/admin/logs          - Get system logs
```

### Telemetry
```
GET    /api/telemetry/workers          - Current telemetry for all workers
GET    /api/telemetry/workers/:id      - Current telemetry for specific worker
WS     /api/telemetry/stream           - WebSocket for live telemetry updates
```

## Technology Stack (Proposed)

### Frontend
- **Framework**: React or Vue.js
- **UI Library**: Material-UI or Ant Design
- **Charts**: Chart.js or Recharts for metrics visualization
- **WebSocket**: Socket.io-client for real-time updates
- **State Management**: Redux or Zustand
- **API Client**: Axios

### Backend (Already Exists)
- **gRPC Server**: Existing Go implementation
- **HTTP/WebSocket Gateway**: Add to master node
- **Authentication**: JWT tokens
- **Database**: MongoDB (already configured)

### Real-time Communication
- **WebSocket Server**: For log streaming and metrics
- **gRPC Streaming**: Backend communication (already planned)
- **Server-Sent Events (SSE)**: Alternative for one-way streaming

## Implementation Priority

### Phase 1: Core API (Current CLI Parity)
1. Task submission endpoint
2. Task listing and details
3. Basic authentication
4. Get stored logs for completed tasks

### Phase 2: Live Monitoring
1. WebSocket endpoint for log streaming
2. Frontend log viewer component
3. Real-time task status updates

### Phase 3: Dashboards
1. User dashboard with task overview
2. Worker status dashboard
3. Resource utilization charts

### Phase 4: Advanced Features
1. Multi-task monitoring
2. Advanced filters and search
3. Historical metrics and analytics
4. Notifications system

### Phase 5: Admin Panel
1. User management interface
2. Worker management interface
3. System logs and monitoring
4. Resource quota management

## Security Considerations

### Authentication
- Secure password hashing (bcrypt)
- JWT token with expiration
- Refresh token mechanism
- Session management

### Authorization
- Role-based access control (RBAC)
- User can only access their own tasks
- Admin role for system management
- API key authentication for programmatic access

### Data Protection
- HTTPS/TLS for all communications
- Encrypted WebSocket connections (WSS)
- Input validation and sanitization
- Rate limiting to prevent abuse

### Task Isolation
- Users cannot view other users' tasks
- Workers isolated per tenant (future multi-tenancy)
- Secure container execution
- Resource quota enforcement

## Future Enhancements

### Advanced Scheduling
- Cron-like scheduled tasks
- Task dependencies and workflows
- Priority-based scheduling

### Collaboration
- Shared tasks between users
- Team workspaces
- Task templates

### Cost Management
- Resource usage billing
- Cost estimation before task submission
- Budget alerts

### Integrations
- GitHub Actions integration
- CI/CD pipeline triggers
- Slack/Email notifications
- Monitoring tool integrations (Prometheus, Grafana)

---

*This document will be updated as new features are planned and implemented.*
