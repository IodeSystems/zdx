import { useState } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import { useLogin, useRegister, useTokenLogin } from '../api'

export function AuthPage() {
  const [tab, setTab] = useState<'login' | 'token' | 'register'>('login')

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', bgcolor: 'background.default' }}>
      <Box sx={{ width: '100%', maxWidth: 400, p: 3 }}>
        <Typography variant="h5" sx={{ mb: 3, fontWeight: 600 }}>
          zdx
        </Typography>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3 }}>
          <Tab label="Login" value="login" />
          <Tab label="Token" value="token" />
          <Tab label="Register" value="register" />
        </Tabs>
        {tab === 'login' && <LoginTab />}
        {tab === 'token' && <TokenTab />}
        {tab === 'register' && <RegisterTab />}
      </Box>
    </Box>
  )
}

function LoginTab() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const { mutate, isPending, error } = useLogin()

  return (
    <Box component="form" onSubmit={(e) => { e.preventDefault(); mutate({ email, password }) }}>
      <TextField fullWidth label="Email" type="email" value={email} onChange={e => setEmail(e.target.value)} sx={{ mb: 2 }} required />
      <TextField fullWidth label="Password" type="password" value={password} onChange={e => setPassword(e.target.value)} sx={{ mb: 2 }} required />
      {error && <Typography color="error" variant="body2" sx={{ mb: 2 }}>{error.message}</Typography>}
      <Button fullWidth variant="contained" type="submit" disabled={isPending}>
        {isPending ? <CircularProgress size={20} /> : 'Log in'}
      </Button>
    </Box>
  )
}

function TokenTab() {
  const [token, setToken] = useState('')
  const { mutate, isPending, error } = useTokenLogin()

  return (
    <Box component="form" onSubmit={(e) => { e.preventDefault(); mutate(token.trim()) }}>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Paste your API key token directly (e.g. from <code>.zdx/credentials</code>).
      </Typography>
      <TextField fullWidth label="API Key Token" value={token} onChange={e => setToken(e.target.value)} sx={{ mb: 2 }} required multiline minRows={2} />
      {error && <Typography color="error" variant="body2" sx={{ mb: 2 }}>{error.message}</Typography>}
      <Button fullWidth variant="contained" type="submit" disabled={isPending}>
        {isPending ? <CircularProgress size={20} /> : 'Use token'}
      </Button>
    </Box>
  )
}

function RegisterTab() {
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const { mutate, isPending, error } = useRegister()

  return (
    <Box component="form" onSubmit={(e) => { e.preventDefault(); mutate({ email, name, password, invite_code: inviteCode }) }}>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Requires an invite code. Using an admin token creates an admin account.
      </Typography>
      <TextField fullWidth label="Email" type="email" value={email} onChange={e => setEmail(e.target.value)} sx={{ mb: 2 }} required />
      <TextField fullWidth label="Name" value={name} onChange={e => setName(e.target.value)} sx={{ mb: 2 }} required />
      <TextField fullWidth label="Password" type="password" value={password} onChange={e => setPassword(e.target.value)} sx={{ mb: 2 }} required />
      <TextField fullWidth label="Invite Code" value={inviteCode} onChange={e => setInviteCode(e.target.value)} sx={{ mb: 2 }} required />
      {error && <Typography color="error" variant="body2" sx={{ mb: 2 }}>{error.message}</Typography>}
      <Button fullWidth variant="contained" type="submit" disabled={isPending}>
        {isPending ? <CircularProgress size={20} /> : 'Register'}
      </Button>
    </Box>
  )
}
