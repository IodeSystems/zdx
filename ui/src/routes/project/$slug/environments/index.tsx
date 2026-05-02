import { createFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Snackbar,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Science as ScienceIcon,
  RocketLaunch as RocketLaunchIcon,
  Sync as SyncIcon,
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

function formatDriftAge(unixSecs: number): string {
  if (!unixSecs) return ''
  const diffSecs = Math.floor(Date.now() / 1000) - unixSecs
  const days = Math.floor(diffSecs / 86400)
  const hrs = Math.floor(diffSecs / 3600)
  if (days > 0) return `${days}d`
  if (hrs > 0) return `${hrs}h`
  return '<1h'
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

function EnvironmentCard({ slug, env }: { slug: string; env: EnvironmentItem }) {
  const request = useRequestEnvironmentTodo()
  const del = useDeleteEnvironment()
  const [toast, setToast] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [confirming, setConfirming] = useState(false)

  const handleRequest = (kind: 'test' | 'ship' | 'sync') => {
    request.mutate({ slug, name: env.name, kind }, {
      onSuccess: () => setToast(`${kind === 'test' ? 'Test' : kind === 'sync' ? 'Sync' : 'Ship'} todo created for ${env.name}`),
      onError: (e) => setToast(`Error: ${e.message}`),
    })
  }

  const handleConfirmDelete = () => {
    del.mutate({ slug, name: env.name }, {
      onError: (e) => setToast(`Error: ${e.message}`),
      onSettled: () => setConfirming(false),
    })
  }

  useEffect(() => {
    if (!confirming) return
    const timer = setTimeout(() => setConfirming(false), 4000)
    return () => clearTimeout(timer)
  }, [confirming])

  const drifted = (env.drift_count ?? 0) > 0

  return (
    <>
      <Card variant="outlined">
        <CardContent sx={{ pb: '12px !important' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', mb: 0.5 }}>
            <Typography sx={{ fontFamily: 'monospace', fontWeight: 700, flex: 1 }}>{env.name}</Typography>
            <Typography variant="caption" color="text.secondary">{formatRelativeTime(env.deployed_at)}</Typography>
          </Box>

          {env.url ? (
            <Typography
              variant="body2"
              component="a"
              href={env.url}
              target="_blank"
              rel="noopener noreferrer"
              sx={{ color: 'text.secondary', fontSize: '0.8rem', wordBreak: 'break-all', display: 'block', mb: 0.75 }}
            >
              {env.url}
            </Typography>
          ) : null}

          <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', alignItems: 'center', mb: 0.75 }}>
            {env.release_branch ? (
              <Chip
                label={env.release_branch}
                size="small"
                variant="outlined"
                sx={{ fontFamily: 'monospace', fontSize: '0.75rem', height: 20 }}
              />
            ) : null}
            {drifted ? (
              <Tooltip title={`release branch is ${env.drift_count} commit${env.drift_count === 1 ? '' : 's'} behind dev, oldest ${formatDriftAge(env.drift_oldest_age_secs ?? 0)} ago`}>
                <Chip
                  label={`${env.drift_count} behind`}
                  size="small"
                  color="warning"
                  variant="filled"
                  sx={{ fontSize: '0.7rem', height: 18 }}
                  data-testid="drift-chip"
                />
              </Tooltip>
            ) : (
              <Box component="span" data-testid="drift-none" sx={{ display: 'none' }} />
            )}
            {env.current_build_sha ? (
              <>
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
              </>
            ) : null}
          </Box>

          <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', alignItems: 'center' }}>
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
            {env.release_branch && (
              <Tooltip title={`Sync main → ${env.release_branch} (test-gated)`}>
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<SyncIcon fontSize="small" />}
                  onClick={() => handleRequest('sync')}
                  disabled={request.isPending}
                  sx={{ fontSize: '0.75rem', py: 0.25 }}
                >
                  Sync
                </Button>
              </Tooltip>
            )}
            <Tooltip title={env.release_branch ? `Deploy from ${env.release_branch}` : 'Create a ship todo for this environment'}>
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
            <Box sx={{ flex: 1 }} />
            <Tooltip title="Edit URL and branch">
              <IconButton size="small" onClick={() => setEditOpen(true)}>
                <EditIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            {confirming ? (
              <>
                <Button
                  size="small"
                  variant="contained"
                  color="error"
                  onClick={handleConfirmDelete}
                  disabled={del.isPending}
                  sx={{ fontSize: '0.75rem', py: 0.25 }}
                  data-testid="env-confirm-delete"
                >
                  Confirm delete
                </Button>
                <Button
                  size="small"
                  onClick={() => setConfirming(false)}
                  disabled={del.isPending}
                  sx={{ fontSize: '0.75rem', py: 0.25 }}
                  data-testid="env-cancel-delete"
                >
                  Cancel
                </Button>
              </>
            ) : (
              <Tooltip title="Delete environment">
                <IconButton
                  size="small"
                  onClick={() => setConfirming(true)}
                  disabled={del.isPending}
                  aria-label="Delete environment"
                  data-testid="env-delete-button"
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
          </Box>
        </CardContent>
      </Card>
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
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
          {envs.map(env => (
            <EnvironmentCard key={env.id} slug={slug} env={env} />
          ))}
        </Box>
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
