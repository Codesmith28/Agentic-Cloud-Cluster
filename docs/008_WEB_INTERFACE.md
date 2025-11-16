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

### Authentication (Planned)
```
POST   /api/auth/login          - User login (NOT IMPLEMENTED)
POST   /api/auth/logout         - User logout (NOT IMPLEMENTED)
POST   /api/auth/register       - User registration (NOT IMPLEMENTED)
GET    /api/auth/profile        - Get current user profile (NOT IMPLEMENTED)
```

### Tasks (✅ IMPLEMENTED)
```
✅ POST   /api/tasks               - Submit new task
✅ GET    /api/tasks               - List all tasks (supports ?status= filter)
✅ GET    /api/tasks/:id           - Get task details
✅ DELETE /api/tasks/:id           - Cancel/delete task
✅ GET    /api/tasks/:id/logs      - Get stored logs for completed task
❌ POST   /api/tasks/:id/retry     - Retry failed task (NOT IMPLEMENTED)
```

### Live Monitoring (Partially Implemented)
```
✅ WS     /ws                      - WebSocket for live telemetry streaming
❌ WS     /api/tasks/:id/stream    - WebSocket for live log streaming (NOT IMPLEMENTED)
❌ GET    /api/tasks/:id/metrics   - Get real-time metrics (NOT IMPLEMENTED)
❌ WS     /api/tasks/:id/metrics/stream - WebSocket for live metrics (NOT IMPLEMENTED)
```

### Workers (✅ IMPLEMENTED - Read-only)
```
✅ GET    /api/workers             - List all workers with metrics
✅ GET    /api/workers/:id         - Get worker details
✅ GET    /api/workers/:id/metrics - Get worker resource metrics
✅ GET    /api/workers/:id/tasks   - Get tasks assigned to worker
```

### Admin Endpoints (Planned)
```
❌ POST   /api/admin/workers       - Register new worker (NOT IMPLEMENTED)
❌ PUT    /api/admin/workers/:id   - Update worker configuration (NOT IMPLEMENTED)
❌ DELETE /api/admin/workers/:id   - Deregister worker (NOT IMPLEMENTED)

❌ GET    /api/admin/users         - List all users (NOT IMPLEMENTED)
❌ POST   /api/admin/users         - Create new user (NOT IMPLEMENTED)
❌ PUT    /api/admin/users/:id     - Update user (NOT IMPLEMENTED)
❌ DELETE /api/admin/users/:id     - Delete user (NOT IMPLEMENTED)

❌ GET    /api/admin/stats         - Get cluster statistics (NOT IMPLEMENTED)
❌ GET    /api/admin/logs          - Get system logs (NOT IMPLEMENTED)
```

### Telemetry (✅ IMPLEMENTED)
```
✅ GET    /telemetry               - Current telemetry for all workers
✅ GET    /telemetry/:id           - Current telemetry for specific worker
✅ GET    /workers                 - All workers basic info
✅ WS     /ws                      - WebSocket for live telemetry updates
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

### Phase 1: Core API ✅ COMPLETE
1. ✅ Task submission endpoint (`POST /api/tasks`)
2. ✅ Task listing and details (`GET /api/tasks`, `GET /api/tasks/:id`)
3. ✅ Task cancellation (`DELETE /api/tasks/:id`)
4. ✅ Get stored logs for completed tasks (`GET /api/tasks/:id/logs`)
5. ✅ Worker listing and details (`GET /api/workers`, `GET /api/workers/:id`)
6. ✅ Worker metrics and tasks (`GET /api/workers/:id/metrics`, `GET /api/workers/:id/tasks`)
7. ❌ Basic authentication (NOT IMPLEMENTED)

### Phase 2: Live Monitoring (Partially Complete)
1. ✅ WebSocket endpoint for telemetry streaming (`/ws`)
2. ❌ WebSocket endpoint for task log streaming
3. ❌ Frontend log viewer component
4. ❌ Real-time task status updates via WebSocket

### Phase 3: Dashboards (Not Started)
1. ❌ User dashboard with task overview
2. ❌ Worker status dashboard
3. ❌ Resource utilization charts

### Phase 4: Advanced Features (Not Started)
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
