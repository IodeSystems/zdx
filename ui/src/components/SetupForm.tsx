import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Box, Button, TextField, Typography, Alert } from '@mui/material'
import { apiFetch } from '../api'

export function SetupForm() {
  const [token, setToken] = useState('')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const queryClient = useQueryClient()

  const setup = useMutation({
    mutationFn: () =>
      apiFetch<{ ok: boolean }>('/api/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, email, name, password }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['setup'] }),
    onError: (err: Error) => setError(err.message),
  })

  return (
    <Box sx={{ maxWidth: 400, mx: 'auto', mt: 10 }}>
      <Typography variant="h5" sx={{ mb: 1 }}>
        zdx setup
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Enter the setup token from the server console to create your admin account.
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      <Box component="form" onSubmit={e => { e.preventDefault(); setup.mutate() }} sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        <TextField label="Setup token" value={token} onChange={e => setToken(e.target.value)} autoFocus size="small" />
        <TextField label="Email" type="email" value={email} onChange={e => setEmail(e.target.value)} size="small" />
        <TextField label="Name" value={name} onChange={e => setName(e.target.value)} size="small" />
        <TextField label="Password" type="password" value={password} onChange={e => setPassword(e.target.value)} size="small" />
        <Button type="submit" variant="contained" disabled={setup.isPending}>
          {setup.isPending ? 'Setting up...' : 'Create admin account'}
        </Button>
      </Box>
    </Box>
  )
}
