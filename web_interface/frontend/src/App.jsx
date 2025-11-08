import React, { useContext } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import Login from './components/Login'
import Dashboard from './components/Dashboard'
import SubmitTask from './components/SubmitTask'
import TaskDetail from './components/TaskDetail'
import AdminUsers from './components/AdminUsers'
import { AuthContext } from './context/AuthContext'

function Protected({ children }) {
  const { token } = useContext(AuthContext)
  if (!token) return <Navigate to="/login" />
  return children
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <Protected>
            <Navigate to="/dashboard" />
          </Protected>
        }
      />
      <Route
        path="/dashboard"
        element={
          <Protected>
            <Dashboard />
          </Protected>
        }
      />
      <Route
        path="/submit"
        element={
          <Protected>
            <SubmitTask />
          </Protected>
        }
      />
      <Route
        path="/tasks/:id"
        element={
          <Protected>
            <TaskDetail />
          </Protected>
        }
      />
      <Route
        path="/admin/users"
        element={
          <Protected>
            <AdminUsers />
          </Protected>
        }
      />
      <Route path="*" element={<Navigate to="/dashboard" />} />
    </Routes>
  )
}
