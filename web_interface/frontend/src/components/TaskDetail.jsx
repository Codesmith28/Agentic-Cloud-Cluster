import React, { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import api from '../api'

export default function TaskDetail() {
  const { id } = useParams()
  const [task, setTask] = useState(null)
  const [logs, setLogs] = useState([])
  const [wsConnected, setWsConnected] = useState(false)

  useEffect(() => {
    api.get(`/tasks/${id}`).then(res => setTask(res.data.task)).catch(() => {})
    api.get(`/tasks/${id}/logs`).then(res => setLogs(res.data.logs || [])).catch(() => {})
  }, [id])

  // Placeholder WebSocket for live streaming (backend not implemented here)
  useEffect(() => {
    let ws
    try {
      ws = new WebSocket(`ws://localhost:5000/api/tasks/${id}/stream`)
      ws.onopen = () => setWsConnected(true)
      ws.onmessage = (e) => {
        try {
          const data = JSON.parse(e.data)
          if (data.line) setLogs(prev => [...prev, data])
        } catch (err) {
          setLogs(prev => [...prev, { ts: new Date().toISOString(), source: 'stdout', line: e.data }])
        }
      }
      ws.onclose = () => setWsConnected(false)
    } catch (err) {
      console.warn('WS not available', err)
    }
    return () => ws && ws.close()
  }, [id])

  if (!task) return <div style={{ padding: 20 }}>Loading...</div>

  return (
    <div style={{ padding: 20 }}>
      <h2>{task.title}</h2>
      <div>Status: {task.status}</div>
      <div>Docker image: {task.dockerImage}</div>
      <div>Command: <pre>{task.command}</pre></div>
      <div>Assigned Worker: {task.assignedWorker || 'n/a'}</div>
      <div style={{ marginTop: 12 }}>
        <h3>Logs {wsConnected ? '(live)' : '(stored)'}:</h3>
        <div style={{ maxHeight: 400, overflow: 'auto', background: '#111', color: '#eee', padding: 8 }}>
          {logs.map((l, i) => (
            <div key={i} style={{ color: l.source === 'stderr' ? 'salmon' : '#ddd' }}>
              [{new Date(l.ts).toLocaleTimeString()}] {l.line}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
