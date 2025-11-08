import React, { useState, useContext } from 'react'
import api from '../api'
import { AuthContext } from '../context/AuthContext'
import { useNavigate } from 'react-router-dom'
import { Container, Stack, Typography, TextField, Button, Alert, CircularProgress } from '@mui/material'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [mode, setMode] = useState('login')
  const [err, setErr] = useState(null)
  const [loading, setLoading] = useState(false)
  const { login, register } = useContext(AuthContext)
  const navigate = useNavigate()

  const submit = async e => {
    e.preventDefault()
    setErr(null)
    setLoading(true)
    try {
      if (mode === 'login') {
        const res = await api.post('/auth/login', { email, password })
        if (res.data && res.data.token) {
          login(res.data.token, res.data.user)
        } else {
          setErr(res.data?.message || 'Login failed')
        }
      } else {
        const res = await register({ name, email, password })
        if (res.data && (res.data.message || res.status === 201)) {
          setMode('login')
          navigate('/login')
        } else setErr(res.data?.message || 'Register failed')
      }
    } catch (e) {
      setErr(e.response?.data?.message || e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Container maxWidth="xs" sx={{ mt: 6 }}>
      <Stack spacing={2}>
        <Typography variant="h5">{mode === 'login' ? 'Login' : 'Register'}</Typography>
        {err && <Alert severity="error">{err}</Alert>}
        {mode === 'register' && <TextField label="Name" value={name} onChange={e => setName(e.target.value)} fullWidth />}
        <TextField label="Email" value={email} onChange={e => setEmail(e.target.value)} fullWidth />
        <TextField label="Password" type="password" value={password} onChange={e => setPassword(e.target.value)} fullWidth />
        <Stack direction="row" spacing={1}>
          <Button variant="contained" onClick={submit} disabled={loading} startIcon={loading && <CircularProgress size={16} />}>
            {mode === 'login' ? 'Login' : 'Create account'}
          </Button>
          <Button variant="outlined" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
            {mode === 'login' ? 'Register' : 'Back to login'}
          </Button>
        </Stack>
      </Stack>
    </Container>
  )
}
