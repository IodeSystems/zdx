import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Paper,
  Snackbar,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Science as ScienceIcon,
  RocketLaunch as RocketLaunchIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  Add as AddIcon,
} from '@mui/icons-material'
import {
  useEnvironments,
  useRequestEnvironmentTodo,
  useCreateEnvironment,
  useUpdateEnvironment,
  useDeleteEnvironment,
} from '../../../../api'
import type { EnvironmentItem } from '../../../../api'

function formatRelativeTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const mins = Math.floor(diffMs / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  return `${days}d ago`
}

function shortSha(sha: string): string {
  if (!sha) return '—'
  return sha.length > 8 ? sha.slice(0, 8) : sha
}

function AddEnvironmentDialog({ slug, open, onClose }: { slug: string; open: boolean; onClose: () => void }) {
  const create = useCreateEnvironment()
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [releaseBranch, setReleaseBranch] = useState('')
  const [error, setError] = useState('')

  const handleSubmit = () => {
    if (!name.trim()) { setError('Name is required'); return }
    create.mutate({ slug, name: name.trim(), url: url.trim(), release_branch: releaseBranch.trim() }, {
      onSuccess: () => { setName(''); setUrl(''); setReleaseBranch(''); setError(''); onClose() },
      onError: (e) => setError(e.message),
    })
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Add Environment</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '12px !important' }}>
        <TextField
          label="Name"
          value={name}
          onChange={e => setName(e.target.value)}
          size="small"
          placeholder="production"
          autoFocus
          error={!!error}
          helperText={error}
        />
        <TextField
          label="URL (optional)"
          value={url}
          onChange={e => setUrl(e.target.value)}
          size="small"
          placeholder="https://example.com"
        />
        <TextField
          label="Release branch (optional)"
          value={releaseBranch}
          onChange={e => setReleaseBranch(e.target.value)}
          size="small"
          placeholder="release/production"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} size="small">Cancel</Button>
        <Button onClick={handleSubmit} variant="contained" size="small" disabled={create.isPending}>Add</Button>
      </DialogActions>
    </Dialog>
  )
}

function EditEnvironmentDialog({ slug, env, open, onClose }: { slug: string; env: EnvironmentItem; open: boolean; onClose: () => void }) {
  const update = useUpdateEnvironment()
  const [url, setUrl] = useState(env.url ?? '')
  const [releaseBranch, setReleaseBranch] = useState(env.release_branch ?? '')
  const [error, setError] = useState('')

  const handleSubmit = () => {
    update.mutate({ slug, name: env.name, url, release_branch: releaseBranch }, {
      onSuccess: () => { setError(''); onClose() },
      onError: (e) => setError(e.message),
    })
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit {env.name}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '12px !important' }}>
        <TextField
          label="URL"
          value={url}
          onChange={e => setUrl(e.target.value)}
          size="small"
          fullWidth
          placeholder="https://example.com"
          autoFocus
          error={!!error}
          helperText={error}
        />
        <TextField
          label="Release branch"
          value={releaseBranch}
          onChange={e => setReleaseBranch(e.target.value)}
          size="small"
          fullWidth
          placeholder="release/production"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} size="small">Cancel</Button>
        <Button onClick={handleSubmit} variant="contained" size="small" disabled={update.isPending}>Save</Button>
      </DialogActions>
    </Dialog>
  )
}

function EnvironmentRow({ slug, env }: { slug: string; env: EnvironmentItem }) {
  const request = useRequestEnvironmentTodo()
  const del = useDeleteEnvironment()
  const [toast, setToast] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)

  const handleRequest = (kind: 'test' | 'ship') => {
    request.mutate({ slug, name: env.name, kind }, {
      onSuccess: () => setToast(`${kind === 'test' ? 'Test' : 'Ship'} todo created for ${env.name}`),
      onError: (e) => setToast(`Error: ${e.message}`),
    })
  }

  const handleDelete = () => {
    if (!confirm(`Delete environment "${env.name}"?`)) return
    del.mutate({ slug, name: env.name }, {
      onError: (e) => setToast(`Error: ${e.message}`),
    })
  }

  return (
    <>
      <TableRow hover>
        <TableCell sx={{ fontFamily: 'monospace', fontWeight: 600 }}>
          {env.name}
        </TableCell>
        <TableCell>
          {env.url ? (
            <Typography variant="body2" component="a" href={env.url} target="_blank" rel="noopener noreferrer"
              sx={{ color: 'text.secondary', fontSize: '0.8rem', wordBreak: 'break-all' }}>
              {env.url}
            </Typography>
          ) : (
            <Typography variant="body2" color="text.disabled">—</Typography>
          )}
        </TableCell>
        <TableCell>
          {env.release_branch ? (
            <Chip
              label={env.release_branch}
              size="small"
              variant="outlined"
              sx={{ fontFamily: 'monospace', fontSize: '0.75rem', height: 20 }}
            />
          ) : (
            <Typography variant="body2" color="text.disabled">—</Typography>
          )}
        </TableCell>
        <TableCell>
          {env.current_build_sha ? (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Chip
                label={shortSha(env.current_build_sha)}
                size="small"
                variant="outlined"
                sx={{ fontFamily: 'monospace', fontSize: '0.75rem', height: 20 }}
              />
              {env.current_build_branch && (
                <Chip
                  label={env.current_build_branch}
                  size="small"
                  color={env.current_build_branch === 'main' || env.current_build_branch === 'master' ? 'primary' : 'default'}
                  variant={env.current_build_branch === 'main' || env.current_build_branch === 'master' ? 'filled' : 'outlined'}
                  sx={{ fontSize: '0.7rem', height: 18 }}
                />
              )}
            </Box>
          ) : (
            <Typography variant="body2" color="text.disabled">not deployed</Typography>
          )}
        </TableCell>
        <TableCell sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
          {formatRelativeTime(env.deployed_at)}
        </TableCell>
        <TableCell>
          <Box sx={{ display: 'flex', gap: 0.5 }}>
            <Tooltip title="Create a test todo for this environment">
              <Button
                size="small"
                variant="outlined"
                startIcon={<ScienceIcon fontSize="small" />}
                onClick={() => handleRequest('test')}
                disabled={request.isPending}
                sx={{ fontSize: '0.75rem', py: 0.25 }}
              >
                Test
              </Button>
            </Tooltip>
            <Tooltip title="Create a ship todo for this environment">
              <Button
                size="small"
                variant="outlined"
                startIcon={<RocketLaunchIcon fontSize="small" />}
                onClick={() => handleRequest('ship')}
                disabled={request.isPending}
                sx={{ fontSize: '0.75rem', py: 0.25 }}
              >
                Ship
              </Button>
            </Tooltip>
            <Tooltip title="Edit URL">
              <IconButton size="small" onClick={() => setEditOpen(true)}>
                <EditIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="Delete environment">
              <IconButton size="small" onClick={handleDelete} disabled={del.isPending}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        </TableCell>
      </TableRow>
      {editOpen && (
        <EditEnvironmentDialog slug={slug} env={env} open={editOpen} onClose={() => setEditOpen(false)} />
      )}
      <Snackbar
        open={!!toast}
        autoHideDuration={3000}
        onClose={() => setToast(null)}
        message={toast}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />
    </>
  )
}

function EnvironmentsPage() {
  const { slug } = Route.useParams()
  const { data: environments, isLoading } = useEnvironments(slug)
  const [addOpen, setAddOpen] = useState(false)
  const envs = environments ?? []

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
        <Typography variant="h6" sx={{ fontWeight: 600, flexGrow: 1 }}>Environments</Typography>
        <Button
          size="small"
          variant="outlined"
          startIcon={<AddIcon fontSize="small" />}
          onClick={() => setAddOpen(true)}
        >
          Add
        </Button>
      </Box>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : envs.length === 0 ? (
        <Box>
          <Typography color="text.secondary" sx={{ mb: 1 }}>No environments registered.</Typography>
          <Typography variant="body2" color="text.secondary">
            Add one above or via{' '}
            <Typography component="span" variant="body2" sx={{ fontFamily: 'monospace' }}>
              POST /api/dx/projects/{slug}/environments
            </Typography>
          </Typography>
        </Box>
      ) : (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 600 }}>Name</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>URL</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Release Branch</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Version</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Deployed</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {envs.map(env => (
                <EnvironmentRow key={env.id} slug={slug} env={env} />
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {envs.length > 0 && (
        <Typography variant="caption" color="text.secondary" sx={{ mt: 2, display: 'block' }}>
          Test/Ship buttons create a todo in the{' '}
          <Link to="/project/$slug/queue" params={{ slug }} style={{ color: 'inherit' }}>
            queue
          </Link>
          {' '}for agent pickup.
        </Typography>
      )}

      <AddEnvironmentDialog slug={slug} open={addOpen} onClose={() => setAddOpen(false)} />
    </>
  )
}

export const Route = createFileRoute('/project/$slug/environments/')({
  component: EnvironmentsPage,
})
