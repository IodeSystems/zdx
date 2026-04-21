import { createFileRoute, useNavigate, Link } from '@tanstack/react-router'
import React, { useState } from 'react'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Paper,
  Select,
  Stack,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Typography,
  FormHelperText,
  FormControl,
  InputLabel,
} from '@mui/material'
import { useProjects, useCreateProject } from '../../api'

const CLASSIFICATIONS = ['library', 'tool', 'service', 'saas', 'site'] as const
type Classification = typeof CLASSIFICATIONS[number]

function slugify(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function repoBasename(url: string): string {
  return url.replace(/\.git$/, '').split('/').filter(Boolean).pop() ?? ''
}

function CreateProjectDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const create = useCreateProject()
  const navigate = useNavigate()
  const [tab, setTab] = useState<0 | 1>(0)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [classification, setClassification] = useState<Classification | ''>('')
  const [upstreamURL, setUpstreamURL] = useState('')
  const [upstreamCredentials, setUpstreamCredentials] = useState('')
  const [localGit, setLocalGit] = useState(false)
  const [createdSlug, setCreatedSlug] = useState<string | null>(null)

  const isBindMode = tab === 1

  const reset = () => {
    setTab(0)
    setName('')
    setSlug('')
    setSlugTouched(false)
    setClassification('')
    setUpstreamURL('')
    setUpstreamCredentials('')
    setLocalGit(false)
    setCreatedSlug(null)
    create.reset()
  }

  const handleClose = () => {
    if (create.isPending) return
    if (createdSlug) {
      navigate({ to: '/project/$slug', params: { slug: createdSlug } })
    }
    reset()
    onClose()
  }

  const handleNameChange = (v: string) => {
    setName(v)
    if (!slugTouched) setSlug(slugify(v))
  }

  const handleUpstreamURLChange = (v: string) => {
    setUpstreamURL(v)
    const base = repoBasename(v)
    if (base) {
      setName(base)
      setSlug(slugify(base))
    }
  }

  const handleTabChange = (_: React.SyntheticEvent, v: 0 | 1) => {
    setTab(v)
    setLocalGit(false)
    setUpstreamURL('')
    create.reset()
  }

  const canSubmit =
    (isBindMode ? (localGit || upstreamURL.trim() !== '') : name.trim() !== '') &&
    slug.trim() !== '' &&
    classification !== '' &&
    !create.isPending

  const handleSubmit = () => {
    if (!canSubmit || !classification) return
    create.mutate(
      {
        slug: slug.trim(),
        name: (isBindMode && !name.trim() ? repoBasename(upstreamURL) : name).trim() || slug.trim(),
        classification,
        ...(upstreamURL.trim() ? { upstream_url: upstreamURL.trim() } : {}),
        ...(upstreamCredentials.trim() ? { upstream_credentials: upstreamCredentials.trim() } : {}),
        ...(localGit && !upstreamURL.trim() ? { local_git: true } : {}),
      },
      {
        onSuccess: (proj) => {
          if (localGit && !upstreamURL.trim()) {
            setCreatedSlug(proj.slug)
          } else {
            reset()
            onClose()
            navigate({ to: '/project/$slug', params: { slug: proj.slug } })
          }
        },
      },
    )
  }

  const serverURL = window.location.origin

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ pb: 0 }}>
        <Tabs value={tab} onChange={handleTabChange} sx={{ mb: 1 }}>
          <Tab label="New Project" />
          <Tab label="Add Existing" />
        </Tabs>
      </DialogTitle>
      <DialogContent>
        {createdSlug ? (
          <Stack spacing={2} sx={{ mt: 1 }}>
            <Typography variant="body2" color="success.main" sx={{ fontWeight: 600 }}>
              Project created with zdx-hosted git.
            </Typography>
            <Typography variant="body2">
              Push your local repo to zdx:
            </Typography>
            <Box component="pre" sx={{ bgcolor: 'action.hover', p: 1.5, borderRadius: 1, fontSize: '0.78rem', overflow: 'auto' }}>
              {`git init\ngit add .\ngit commit -m "init"\ngit remote add origin ${serverURL}/git/${createdSlug}\ngit push -u origin main`}
            </Box>
            <Typography variant="caption" color="text.secondary">
              Use your zdx API key as the password when prompted.
            </Typography>
          </Stack>
        ) : (
          <Stack spacing={2} sx={{ mt: 1 }}>
            {isBindMode ? (
              <>
                <Tabs
                  value={localGit ? 1 : 0}
                  onChange={(_, v) => { setLocalGit(v === 1); setUpstreamURL(''); create.reset() }}
                  variant="fullWidth"
                  sx={{ mb: 0, minHeight: 36, '& .MuiTab-root': { minHeight: 36, py: 0.5, fontSize: '0.8rem' } }}
                >
                  <Tab label="GitHub upstream" />
                  <Tab label="Local (zdx-hosted)" />
                </Tabs>
                {!localGit ? (
                  <TextField
                    label="GitHub URL"
                    value={upstreamURL}
                    onChange={(e) => handleUpstreamURLChange(e.target.value)}
                    placeholder="https://github.com/owner/repo.git"
                    required
                    autoFocus
                    size="small"
                    fullWidth
                    helperText="Bind this repo to zdx and enable git proxy for srcless agents"
                  />
                ) : (
                  <Typography variant="body2" color="text.secondary" sx={{ pt: 0.5 }}>
                    zdx will create a bare git repo on the server. After creation you'll get push instructions.
                  </Typography>
                )}
              </>
            ) : (
              <TextField
                label="Name"
                value={name}
                onChange={(e) => handleNameChange(e.target.value)}
                required
                autoFocus
                size="small"
                fullWidth
              />
            )}
            <TextField
              label="Slug"
              value={slug}
              onChange={(e) => {
                setSlug(slugify(e.target.value))
                setSlugTouched(true)
              }}
              required
              size="small"
              fullWidth
              helperText="URL identifier — auto-derived from name"
            />
            <FormControl size="small" fullWidth required>
              <InputLabel id="classification-label">Classification</InputLabel>
              <Select
                labelId="classification-label"
                label="Classification"
                value={classification}
                onChange={(e) => setClassification(e.target.value as Classification)}
              >
                {CLASSIFICATIONS.map((c) => (
                  <MenuItem key={c} value={c}>{c}</MenuItem>
                ))}
              </Select>
              <FormHelperText>Shapes the maturity vine and doctor checks</FormHelperText>
            </FormControl>
            {!isBindMode && (
              <TextField
                label="GitHub URL (optional)"
                value={upstreamURL}
                onChange={(e) => setUpstreamURL(e.target.value)}
                placeholder="https://github.com/owner/repo.git"
                size="small"
                fullWidth
                helperText="Provide to enable git proxy for srcless agents"
              />
            )}
            {(!isBindMode || (isBindMode && !localGit)) && (
              <TextField
                label="GitHub PAT (optional)"
                type="password"
                value={upstreamCredentials}
                onChange={(e) => setUpstreamCredentials(e.target.value)}
                disabled={upstreamURL.trim() === ''}
                size="small"
                fullWidth
                helperText="Personal access token; stored encrypted server-side"
              />
            )}
            {create.isError && (
              <Typography color="error" variant="body2">
                {create.error.message}
              </Typography>
            )}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={create.isPending}>
          {createdSlug ? 'Done' : 'Cancel'}
        </Button>
        {!createdSlug && (
          <Button variant="contained" onClick={handleSubmit} disabled={!canSubmit}>
            {create.isPending
              ? (isBindMode ? (localGit ? 'Creating…' : 'Binding…') : 'Creating…')
              : (isBindMode ? (localGit ? 'Create with zdx git' : 'Bind Repo') : 'Create')}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  )
}

function ProjectsPage() {
  const { data: projects, isLoading } = useProjects()
  const [open, setOpen] = useState(false)

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  return (
    <Box sx={{ maxWidth: 960 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          Projects
        </Typography>
        <Button variant="contained" onClick={() => setOpen(true)}>
          Create Project
        </Button>
      </Box>
      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Slug</TableCell>
              <TableCell>Classification</TableCell>
              <TableCell>Git</TableCell>
              <TableCell>Created</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(!projects || projects.length === 0) && (
              <TableRow>
                <TableCell colSpan={5} sx={{ textAlign: 'center', color: 'text.secondary' }}>
                  No projects yet
                </TableCell>
              </TableRow>
            )}
            {projects?.map((p) => (
              <TableRow key={p.id} hover>
                <TableCell>
                  <Link
                    to="/project/$slug"
                    params={{ slug: p.slug }}
                    style={{ color: 'inherit', textDecoration: 'none', fontWeight: 500 }}
                  >
                    {p.name}
                  </Link>
                </TableCell>
                <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{p.slug}</TableCell>
                <TableCell>
                  {p.classification ? (
                    <Chip label={p.classification} size="small" />
                  ) : (
                    <Typography variant="caption" color="text.secondary">—</Typography>
                  )}
                </TableCell>
                <TableCell>
                  {p.git_enabled ? <Chip label="enabled" size="small" color="success" /> : <Typography variant="caption" color="text.secondary">—</Typography>}
                </TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>
                  {new Date(p.created_at).toLocaleDateString()}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      <CreateProjectDialog open={open} onClose={() => setOpen(false)} />
    </Box>
  )
}

export const Route = createFileRoute('/admin/projects')({
  component: ProjectsPage,
})
