# CloudAI Frontend UI

Beautiful React-based web interface for the CloudAI distributed task scheduler.

## Features

- **Dashboard** - Real-time cluster overview with task and worker statistics
- **Task Submission** - Submit Docker tasks with resource requirements
- **Task Management** - View, monitor, and cancel tasks
- **Worker Monitoring** - Real-time worker status and resource utilization
- **Task Logs Viewer** - View task execution logs in real-time
- **Authentication** - User registration and JWT-based login
- **Worker Registration** - Register new workers via UI
- **Enhanced Task Model** - Tag classification and K-value priority system

## New Task Model Fields

### 1. Task Tag (Required)
Classify your task workload type for optimized scheduling:
- **cpu-light** - Light CPU workload
- **cpu-heavy** - Intensive CPU workload
- **memory-light** - Light memory usage
- **memory-heavy** - High memory usage
- **gpu-training** - GPU-based training
- **mixed** - Mixed resource requirements

### 2. K-Value (Required)
Scheduling priority multiplier (1.5 - 2.5):
- **1.5** - Low priority
- **2.0** - Normal priority (default)
- **2.5** - High priority

Adjustable in 0.1 increments using a slider.

## Tech Stack

- **React 18.2** - UI framework
- **Material-UI 5.14** - Component library
- **Vite 5.0** - Build tool
- **Axios 1.5** - HTTP client
- **Recharts 2.8** - Charts (for future enhancements)
- **React Router 6.20** - Routing

## Prerequisites

- Node.js 18+ and npm
- CloudAI backend running on `http://localhost:8080`

## Installation

```bash
# Navigate to UI folder
cd ui

# Install dependencies
npm install

# Start development server
npm run dev
```

The app will be available at `http://localhost:3000`

## Build for Production

```bash
npm run build
npm run preview  # Preview production build
```

## API Configuration

Edit `.env` file to configure backend URLs:

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_BASE_URL=ws://localhost:8080
```

## Project Structure

```
ui/
├── src/
│   ├── api/           # API clients
│   │   ├── auth.js    # Authentication API
│   │   ├── client.js  # Base HTTP client
│   │   ├── tasks.js   # Task API
│   │   ├── websocket.js # WebSocket client
│   │   └── workers.js # Worker API
│   ├── components/
│   │   ├── auth/      # Auth components
│   │   │   └── ProtectedRoute.jsx
│   │   ├── layout/    # Layout components
│   │   │   ├── Navbar.jsx
│   │   │   └── Sidebar.jsx
│   │   ├── tasks/     # Task components
│   │   │   ├── SubmitTask.jsx
│   │   │   └── TaskLogsDialog.jsx
│   │   └── WorkerRegistrationDialog.jsx
│   ├── context/
│   │   └── AuthContext.jsx  # Auth state management
│   ├── hooks/         # Custom React hooks
│   │   ├── useRealTimeTasks.js
│   │   ├── useTelemetry.js
│   │   └── useWebSocket.js
│   ├── pages/
│   │   ├── Dashboard.jsx
│   │   ├── TasksPage.jsx
│   │   ├── WorkersPage.jsx
│   │   ├── SubmitTaskPage.jsx
│   │   └── auth/
│   │       ├── LoginPage.jsx
│   │       └── RegisterPage.jsx
│   ├── utils/         # Constants, formatters
│   ├── styles/        # Global CSS
│   ├── App.jsx        # Main app with routing
│   └── main.jsx       # Entry point
├── package.json
├── vite.config.js
└── .env
```

## Usage

### Submit a Task

1. Click "Submit Task" in sidebar
2. Fill in:
   - Docker Image (required)
   - Command (optional)
   - **Task Tag** (required) - Select workload type
   - **K-Value** (required) - Adjust priority slider
   - CPU, Memory, Storage, GPU requirements
3. Click "Submit Task"

### View Tasks

- Navigate to "Tasks" page
- See all tasks with status, tag, k-value
- Cancel running tasks
- Auto-refreshes every 5 seconds

### Monitor Workers

- Navigate to "Workers" page
- View worker status and resource usage
- See CPU/Memory utilization bars
- Auto-refreshes every 5 seconds

## API Endpoints Used

### Authentication
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login (returns JWT token)

### Tasks
- `POST /api/tasks` - Submit task (with tag & k_value)
- `GET /api/tasks` - List all tasks
- `GET /api/tasks/:id` - Get task details
- `DELETE /api/tasks/:id` - Cancel task

### Workers
- `GET /api/workers` - List all workers
- `GET /api/workers/:id` - Get worker details

### Real-time
- `WS /ws/telemetry` - Real-time telemetry updates

## Future Enhancements

- Resource usage charts (using Recharts)
- Worker details page with metrics
- Task filtering and search
- Dark mode support
- Export task history

## Troubleshooting

### Backend Connection Failed
- Ensure CloudAI master is running on port 8080
- Check CORS settings in master node
- Verify `.env` configuration

### Build Errors
```bash
rm -rf node_modules package-lock.json
npm install
```

### Port Already in Use
Edit `vite.config.js` to change port:
```js
server: {
  port: 3001, // Change port
}
```

## License

Same as CloudAI project

## Contributing

See main CloudAI repository for contribution guidelines
