import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Box, Button, TextField, Typography, Alert } from '@mui/material'
import { apiFetch } from '../api'

export function LoginForm() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const queryClient = useQueryClient()

  const login = useMutation({
    mutationFn: () =>
      apiFetch<{ ok: boolean }>('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['me'] }),
    onError: (err: Error) => setError(err.message),
  })

  return (
    <Box sx={{ maxWidth: 360, mx: 'auto', mt: 10 }}>
      <Typography variant="h5" sx={{ mb: 3 }}>
        Sign in
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      <Box
        component="form"
        onSubmit={e => { e.preventDefault(); login.mutate() }}
        sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}
      >
        <TextField
          label="Email"
          type="email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          autoFocus
          size="small"
        />
        <TextField
          label="Password"
          type="password"
          value={password}
          onChange={e => setPassword(e.target.value)}
          size="small"
        />
        <Button type="submit" variant="contained" disabled={login.isPending}>
          {login.isPending ? 'Signing in...' : 'Sign in'}
        </Button>
      </Box>
    </Box>
  )
}
