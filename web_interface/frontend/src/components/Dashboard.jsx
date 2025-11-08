import React, { useEffect, useState, useContext } from 'react'
import { Link as RouterLink } from 'react-router-dom'
import { AuthContext } from '../context/AuthContext'
import {
  Container,
  Typography,
  Box,
  Button,
  TextField,
  Select,
  MenuItem,
  InputLabel,
  FormControl,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  Paper,
  Stack,
  Alert
} from '@mui/material'

export default function Dashboard() {
  const { user, logout, api } = useContext(AuthContext)
  const [tasks, setTasks] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  // filters and sorting
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [sort, setSort] = useState('createdAt:desc')

  const fetchTasks = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get('/tasks', { params: { q: q || undefined, status: status || undefined, sort } })
      setTasks(res.data.tasks || [])
    } catch (err) {
      setError(err.response?.data?.message || err.message)
      setTasks([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTasks()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q, status, sort])

  return (
    <Container sx={{ mt: 4 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center">
        <Typography variant="h5">Welcome, {user?.name} ({user?.role})</Typography>
        <Stack direction="row" spacing={1}>
          <Button component={RouterLink} to="/submit" variant="contained">Submit Task</Button>
          <Button variant="outlined" onClick={() => { logout(); }}>Logout</Button>
        </Stack>
      </Box>

      <Paper sx={{ mt: 3, p: 2 }}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems="center">
          <TextField label="Search by name or id" value={q} onChange={e => setQ(e.target.value)} size="small" />
          <FormControl size="small" sx={{ minWidth: 150 }}>
            <InputLabel>Status</InputLabel>
            <Select value={status} label="Status" onChange={e => setStatus(e.target.value)}>
              <MenuItem value="">All</MenuItem>
              <MenuItem value="pending">Pending</MenuItem>
              <MenuItem value="running">Running</MenuItem>
              <MenuItem value="completed">Completed</MenuItem>
              <MenuItem value="failed">Failed</MenuItem>
              <MenuItem value="cancelled">Cancelled</MenuItem>
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <InputLabel>Sort</InputLabel>
            <Select value={sort} label="Sort" onChange={e => setSort(e.target.value)}>
              <MenuItem value="createdAt:desc">Created (new→old)</MenuItem>
              <MenuItem value="createdAt:asc">Created (old→new)</MenuItem>
              <MenuItem value="status:asc">Status (A→Z)</MenuItem>
              <MenuItem value="runtimeSeconds:desc">Runtime (long→short)</MenuItem>
            </Select>
          </FormControl>
          <Button onClick={fetchTasks} variant="outlined">Refresh</Button>
        </Stack>

        {error && <Alert severity="error" sx={{ mt: 2 }}>{error}</Alert>}

        <Table sx={{ mt: 2 }}>
          <TableHead>
            <TableRow>
              <TableCell>Task ID</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Docker Image</TableCell>
              <TableCell>Worker</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Runtime</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {tasks.map(t => (
              <TableRow key={t._id} hover>
                <TableCell>
                  <RouterLink to={`/tasks/${t._id}`}>{t._id}</RouterLink>
                </TableCell>
                <TableCell>{t.title || t.name}</TableCell>
                <TableCell>{t.status}</TableCell>
                <TableCell>{t.dockerImage || '-'}</TableCell>
                <TableCell>{t.assignedWorker || '-'}</TableCell>
                <TableCell>{new Date(t.createdAt).toLocaleString()}</TableCell>
                <TableCell>{t.runtimeSeconds ? `${t.runtimeSeconds}s` : '-'}</TableCell>
                <TableCell>
                  <Stack direction="row" spacing={1}>
                    <Button size="small" component={RouterLink} to={`/tasks/${t._id}`}>View</Button>
                    <Button size="small" onClick={async () => {
                      try {
                        const res = await api.post(`/tasks/${t._id}/duplicate`)
                        if (res.data && res.data.task) {
                          // refresh list or navigate to new task
                          setTasks(prev => [res.data.task, ...prev])
                        }
                      } catch (err) {
                        console.error('Duplicate failed', err)
                      }
                    }}>Duplicate</Button>
                    {t.status === 'failed' && (
                      <Button size="small" onClick={async () => {
                        try {
                          const res = await api.post(`/tasks/${t._id}/retry`)
                          if (res.data) {
                            // refresh list
                            fetchTasks()
                          }
                        } catch (err) {
                          console.error('Retry failed', err)
                        }
                      }}>Retry</Button>
                    )}
                  </Stack>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Paper>
    </Container>
  )
}
