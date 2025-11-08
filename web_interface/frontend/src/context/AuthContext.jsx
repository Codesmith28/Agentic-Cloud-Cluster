import React, { createContext, useState, useEffect } from 'react'
import api from '../api'
import { useNavigate } from 'react-router-dom'

export const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(() => JSON.parse(localStorage.getItem('user') || 'null'))
  const [token, setToken] = useState(() => localStorage.getItem('token'))
  const navigate = useNavigate()

  useEffect(() => {
    if (token) localStorage.setItem('token', token)
    else localStorage.removeItem('token')
  }, [token])

  useEffect(() => {
    if (user) localStorage.setItem('user', JSON.stringify(user))
    else localStorage.removeItem('user')
  }, [user])

  const login = (token, user) => {
    setToken(token)
    setUser(user)
    navigate('/dashboard')
  }

  const logout = () => {
    setToken(null)
    setUser(null)
    navigate('/login')
  }

  const register = async (payload) => api.post('/auth/register', payload)

  const value = { user, token, login, logout, register, api }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
