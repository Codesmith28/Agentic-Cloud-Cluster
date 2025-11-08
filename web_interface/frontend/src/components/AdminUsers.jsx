import React, { useEffect, useState } from 'react'
import api from '../api'

export default function AdminUsers(){
  const [users, setUsers] = useState([])

  useEffect(()=>{
    api.get('/admin/users').then(res=> setUsers(res.data.users || [])).catch(()=>{})
  },[])

  return (
    <div style={{ padding: 20 }}>
      <h2>Users</h2>
      <ul>
        {users.map(u => (
          <li key={u._id}>{u.name} ({u.email}) - {u.role}</li>
        ))}
      </ul>
    </div>
  )
}
