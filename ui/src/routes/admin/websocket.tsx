import { createFileRoute } from '@tanstack/react-router'
import { useCallback, useId, useRef, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { useChannel } from '../../hooks/useChannel'
import { useWSClients, useWSEcho } from '../../api'

type EchoResult = {
  sentAt: number
  receivedAt: number
  payload: unknown
}

function WebSocketDiagnostics() {
  const { data: clients, isLoading, error } = useWSClients(2000)
  const echo = useWSEcho()

  const testChannel = `admin:echo:${useId()}`
  const [customPayload, setCustomPayload] = useState('hello')
  const [result, setResult] = useState<EchoResult | null>(null)
  const [echoError, setEchoError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const inFlightRef = useRef<{ sentAt: number } | null>(null)

  const onMessage = useCallback((msg: { type: string; payload: unknown }) => {
    if (msg.type !== 'admin.echo') return
    const active = inFlightRef.current
    if (!active) return
    inFlightRef.current = null
    setPending(false)
    setResult({
      sentAt: active.sentAt,
      receivedAt: Date.now(),
      payload: msg.payload,
    })
  }, [])

  useChannel(testChannel, onMessage)

  const handlePing = () => {
    if (!testChannel) return
    setEchoError(null)
    setResult(null)
    setPending(true)
    const sentAt = Date.now()
    inFlightRef.current = { sentAt }
    echo.mutate(
      { channel: testChannel, payload: customPayload },
      {
        onError: (err) => {
          inFlightRef.current = null
          setPending(false)
          setEchoError(err.message)
        },
      },
    )
  }

  const latencyMs = result ? result.receivedAt - result.sentAt : null

  return (
    <Box sx={{ maxWidth: 960 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        WebSocket Diagnostics
      </Typography>

      <Paper variant="outlined" sx={{ p: 2, mb: 3 }}>
        <Typography variant="h6" sx={{ mb: 1 }}>Ping / Echo</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Subscribed to channel <code>{testChannel}</code>. Press Ping to publish an echo and measure
          round-trip latency.
        </Typography>
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', mb: 2 }}>
          <TextField
            size="small"
            label="Payload"
            value={customPayload}
            onChange={(e) => setCustomPayload(e.target.value)}
            sx={{ flexGrow: 1 }}
          />
          <Button variant="contained" onClick={handlePing} disabled={echo.isPending || !testChannel}>
            {echo.isPending ? 'Sending…' : 'Ping'}
          </Button>
        </Box>
        {echoError && <Alert severity="error" sx={{ mb: 2 }}>{echoError}</Alert>}
        {result && (
          <Alert severity="success">
            Round-trip: <strong>{latencyMs} ms</strong> — payload:{' '}
            <code>{JSON.stringify(result.payload)}</code>
          </Alert>
        )}
        {pending && !result && !echoError && (
          <Alert severity="info">Waiting for echo…</Alert>
        )}
      </Paper>

      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="h6" sx={{ mb: 1 }}>
          Connected Clients {clients ? `(${clients.length})` : ''}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Live WebSocket subscribers. Refreshes every 2s.
        </Typography>
        {isLoading && <CircularProgress size={20} />}
        {error && <Alert severity="error">{error.message}</Alert>}
        {clients && clients.length === 0 && (
          <Typography variant="body2" color="text.secondary">No subscribers.</Typography>
        )}
        {clients && clients.length > 0 && (
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>ID</TableCell>
                <TableCell>Channels</TableCell>
                <TableCell>User</TableCell>
                <TableCell>Remote</TableCell>
                <TableCell>Connected</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {clients.map((c) => (
                <TableRow key={c.id}>
                  <TableCell>{c.id}</TableCell>
                  <TableCell>
                    {c.channels && c.channels.length > 0 ? (
                      c.channels.map((ch) => (
                        <div key={ch}><code>{ch}</code></div>
                      ))
                    ) : (
                      <em>(none)</em>
                    )}
                  </TableCell>
                  <TableCell>{c.user_id || '—'}</TableCell>
                  <TableCell>{c.remote_addr}</TableCell>
                  <TableCell>{new Date(c.connected_at).toLocaleTimeString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </Paper>
    </Box>
  )
}

export const Route = createFileRoute('/admin/websocket')({
  component: WebSocketDiagnostics,
})
