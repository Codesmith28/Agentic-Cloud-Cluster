import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Box } from '@mui/material';
import { AuthProvider } from './context/AuthContext';
import ProtectedRoute from './components/auth/ProtectedRoute';
import Navbar from './components/layout/Navbar';
import Sidebar from './components/layout/Sidebar';
import Dashboard from './pages/Dashboard';
import TasksPage from './pages/TasksPage';
import WorkersPage from './pages/WorkersPage';
import SubmitTaskPage from './pages/SubmitTaskPage';
import LoginPage from './pages/auth/LoginPage';
import RegisterPage from './pages/auth/RegisterPage';

function App() {
  const [sidebarOpen, setSidebarOpen] = React.useState(true);

  const toggleSidebar = () => {
    setSidebarOpen(!sidebarOpen);
  };

  return (
    <AuthProvider>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />

        {/* Protected routes */}
        <Route
          path="/*"
          element={
            <ProtectedRoute>
              <Box sx={{ display: 'flex', minHeight: '100vh', overflow: 'auto' }}>
                <Navbar onMenuClick={toggleSidebar} />
                <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
                
                <Box
                  component="main"
                  sx={{
                    flexGrow: 1,
                    p: 3,
                    mt: 8,
                    width: '100%',
                    overflow: 'auto',
                  }}
                >
                  <Routes>
                    <Route path="/" element={<Navigate to="/dashboard" replace />} />
                    <Route path="/dashboard" element={<Dashboard />} />
                    <Route path="/tasks" element={<TasksPage />} />
                    <Route path="/workers" element={<WorkersPage />} />
                    <Route path="/submit" element={<SubmitTaskPage />} />
                  </Routes>
                </Box>
              </Box>
            </ProtectedRoute>
          }
        />
      </Routes>
    </AuthProvider>
  );
}

export default App;
