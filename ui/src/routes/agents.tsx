import { createFileRoute } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  MenuItem,
  Paper,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  PauseCircleOutlined as PauseIcon,
  PlayCircleOutlined as ResumeIcon,
  Stop as DrainIcon,
  Link as LinkIcon,
  LinkOff as UnlinkIcon,
} from '@mui/icons-material'
import { useState } from 'react'
import {
  useAllAgents,
  useAssignAgent,
  useUnassignAgent,
  useAgentCommand,
  useProjects,
  type AgentItem,
} from '../api'

function relativeTime(iso: string): string {
  if (!iso) return ''
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ''
  const delta = Math.floor((Date.now() - t) / 1000)
  if (delta < 60) return `${delta}s ago`
  if (delta < 3600) return `${Math.floor(delta / 60)}m ago`
  if (delta < 86400) return `${Math.floor(delta / 3600)}h ago`
  return `${Math.floor(delta / 86400)}d ago`
}

function ScopeCell({ a }: { a: AgentItem }) {
  if (a.project_slug) {
    return (
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <Typography variant="body2">{a.project_name || a.project_slug}</Typography>
        {a.originally_global && <Chip size="small" label="pinned" variant="outlined" />}
      </Stack>
    )
  }
  return <Chip size="small" label="global" color="primary" variant="outlined" />
}

function StatusChip({ a }: { a: AgentItem }) {
  const state = a.connection_state
  const color: 'success' | 'warning' | 'default' | 'error' =
    state === 'connected' ? 'success' :
    state === 'paused' ? 'warning' :
    state === 'draining' ? 'warning' :
    'default'
  const label = a.idle && state === 'connected' ? 'idle' : state
  return <Chip size="small" label={label} color={color} variant="outlined" />
}

function PinControls({ a }: { a: AgentItem }) {
  const projects = useProjects()
  const assign = useAssignAgent()
  const unassign = useUnassignAgent()
  const [picking, setPicking] = useState(false)
  const [target, setTarget] = useState('')

  if (!a.originally_global) {
    return <Typography variant="caption" color="text.secondary">scope-immutable</Typography>
  }

  if (a.project_id != null) {
    return (
      <Tooltip title="Unpin from project (back to global pool)">
        <span>
          <IconButton size="small" onClick={() => unassign.mutate(a.id)} disabled={unassign.isPending}>
            <UnlinkIcon fontSize="small" />
          </IconButton>
        </span>
      </Tooltip>
    )
  }

  if (!picking) {
    return (
      <Tooltip title="Pin to a project">
        <span>
          <IconButton size="small" onClick={() => setPicking(true)}>
            <LinkIcon fontSize="small" />
          </IconButton>
        </span>
      </Tooltip>
    )
  }

  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
      <Select
        size="small"
        value={target}
        displayEmpty
        onChange={e => setTarget(e.target.value)}
        sx={{ minWidth: 140 }}
      >
        <MenuItem value="" disabled>project…</MenuItem>
        {(projects.data ?? []).map(p => (
          <MenuItem key={p.slug} value={p.slug}>{p.name}</MenuItem>
        ))}
      </Select>
      <Button
        size="small"
        variant="contained"
        disabled={!target || assign.isPending}
        onClick={() => {
          assign.mutate({ id: a.id, project_slug: target }, {
            onSuccess: () => { setPicking(false); setTarget('') },
          })
        }}
      >
        Pin
      </Button>
      <Button size="small" onClick={() => { setPicking(false); setTarget('') }}>Cancel</Button>
    </Stack>
  )
}

function CommandControls({ a }: { a: AgentItem }) {
  const cmd = useAgentCommand()
  const isPaused = a.status === 'paused'
  const isDraining = a.status === 'draining'
  const isConnected = a.connection_state === 'connected' || isPaused || isDraining
  if (!isConnected) {
    return <Typography variant="caption" color="text.secondary">offline</Typography>
  }
  return (
    <Stack direction="row" spacing={0.5}>
      {isPaused ? (
        <Tooltip title="Resume work loop">
          <span>
            <IconButton size="small" onClick={() => cmd.mutate({ id: a.id, command: 'resume' })} disabled={cmd.isPending}>
              <ResumeIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      ) : (
        <Tooltip title="Pause work loop">
          <span>
            <IconButton size="small" onClick={() => cmd.mutate({ id: a.id, command: 'pause' })} disabled={cmd.isPending || isDraining}>
              <PauseIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      )}
      <Tooltip title="Drain (finish current task, then stop)">
        <span>
          <IconButton size="small" onClick={() => cmd.mutate({ id: a.id, command: 'drain' })} disabled={cmd.isPending || isDraining}>
            <DrainIcon fontSize="small" />
          </IconButton>
        </span>
      </Tooltip>
    </Stack>
  )
}

function AgentsPage() {
  const { data, isLoading, error } = useAllAgents()
  const agents = data?.agents ?? []

  return (
    <Box sx={{ maxWidth: 1200 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 2 }}>
        Agents
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        Every connected agent across the server: project-scoped and global pool.
        Pin global agents to a project to direct work; pause/resume/drain to
        manage the work loop.
      </Typography>

      {isLoading && <CircularProgress size={20} />}
      {error && <Alert severity="error">{String(error.message ?? error)}</Alert>}

      {!isLoading && agents.length === 0 && (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Typography variant="body2" color="text.secondary">
            No agents connected. Run <code>dx agent connect --provider=claude</code> from
            a project, or <code>dx agent connect --provider=claude --global</code> to
            register into the global pool.
          </Typography>
        </Paper>
      )}

      {agents.length > 0 && (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Alias</TableCell>
                <TableCell>Scope</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Connected</TableCell>
                <TableCell>Last heartbeat</TableCell>
                <TableCell>Pin</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {agents.map(a => (
                <TableRow key={a.id} hover>
                  <TableCell sx={{ fontFamily: 'monospace' }}>{a.id}</TableCell>
                  <TableCell><ScopeCell a={a} /></TableCell>
                  <TableCell><StatusChip a={a} /></TableCell>
                  <TableCell>{a.connected_at ? relativeTime(a.connected_at) : ''}</TableCell>
                  <TableCell>{relativeTime(a.last_heartbeat)}</TableCell>
                  <TableCell><PinControls a={a} /></TableCell>
                  <TableCell><CommandControls a={a} /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  )
}

export const Route = createFileRoute('/agents')({
  component: AgentsPage,
})
