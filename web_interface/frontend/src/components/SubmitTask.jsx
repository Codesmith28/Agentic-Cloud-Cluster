import React, { useState, useEffect } from 'react'
import api from '../api'
import { useNavigate } from 'react-router-dom'
import { Container, TextField, Button, Stack, Typography, Alert, Paper } from '@mui/material'

function deriveName({ name, dockerImage, command }) {
  if (name && name.trim()) return name
  if (dockerImage) {
    // derive image base name without registry and tag
    const parts = dockerImage.split('/')
    const last = parts[parts.length - 1]
    const imageName = last.split(':')[0]
    return imageName
  }
  if (command) {
    const first = command.trim().split(/\s+/)[0]
    return first
  }
  return ''
}

export default function SubmitTask() {
  const [name, setName] = useState('')
  const [dockerImage, setDockerImage] = useState('')
  const [command, setCommand] = useState('')
  const [cpu, setCpu] = useState(1)
  const [memory, setMemory] = useState('512Mi')
  const [storage, setStorage] = useState('1Gi')
  const [gpu, setGpu] = useState(0)
  const [err, setErr] = useState(null)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  // auto-derive name when user hasn't provided one explicitly
  useEffect(() => {
    if (!name) {
      const suggested = deriveName({ name, dockerImage, command })
      if (suggested) setName(suggested)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dockerImage, command])

  const submit = async e => {
    e.preventDefault()
    setErr(null)
    // basic client-side validation
    if (!dockerImage && !command) {
      setErr('Either Docker image or command must be provided')
      return
    }
    setLoading(true)
    try {
      const payload = {
        name,
        title: name,
        dockerImage,
        command,
        resources: { cpu: Number(cpu) || 1, memory: memory || '512Mi', storage: storage || '1Gi', gpu: Number(gpu) || 0 }
      }
      const res = await api.post('/tasks', payload)
      if (res.data && res.data.task) {
        navigate(`/tasks/${res.data.task._id}`)
      } else {
        setErr(res.data?.message || 'Submission failed')
      }
    } catch (e) {
      setErr(e.response?.data?.message || e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Container maxWidth="md" sx={{ mt: 4 }}>
      <Paper sx={{ p: 3 }}>
        <Typography variant="h6">Submit New Task</Typography>
        <form onSubmit={submit}>
          <Stack spacing={2} sx={{ mt: 2 }}>
            {err && <Alert severity="error">{err}</Alert>}
            <TextField label="Task name" value={name} onChange={e => setName(e.target.value)} fullWidth />
            <TextField label="Docker image (e.g. ubuntu:latest)" value={dockerImage} onChange={e => { setDockerImage(e.target.value); }} fullWidth />
            <TextField label="Command" value={command} onChange={e => setCommand(e.target.value)} multiline rows={3} fullWidth />
            <Stack direction="row" spacing={2}>
              <TextField label="CPU" type="number" value={cpu} onChange={e => setCpu(e.target.value)} sx={{ width: 120 }} />
              <TextField label="Memory" value={memory} onChange={e => setMemory(e.target.value)} sx={{ width: 160 }} />
              <TextField label="Storage" value={storage} onChange={e => setStorage(e.target.value)} sx={{ width: 160 }} />
              <TextField label="GPU" type="number" value={gpu} onChange={e => setGpu(e.target.value)} sx={{ width: 120 }} />
            </Stack>
            <Stack direction="row" spacing={1}>
              <Button type="submit" variant="contained" disabled={loading}>{loading ? 'Submitting...' : 'Submit Task'}</Button>
              <Button variant="outlined" onClick={() => navigate(-1)}>Cancel</Button>
            </Stack>
          </Stack>
        </form>
      </Paper>
    </Container>
  )
}
