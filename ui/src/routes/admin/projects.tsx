import { createFileRoute, useNavigate, Link } from '@tanstack/react-router'
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

function CreateProjectDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const create = useCreateProject()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [classification, setClassification] = useState<Classification | ''>('')
  const [upstreamURL, setUpstreamURL] = useState('')
  const [upstreamCredentials, setUpstreamCredentials] = useState('')

  const reset = () => {
    setName('')
    setSlug('')
    setSlugTouched(false)
    setClassification('')
    setUpstreamURL('')
    setUpstreamCredentials('')
    create.reset()
  }

  const handleClose = () => {
    if (create.isPending) return
    reset()
    onClose()
  }

  const handleNameChange = (v: string) => {
    setName(v)
    if (!slugTouched) setSlug(slugify(v))
  }

  const canSubmit =
    name.trim() !== '' &&
    slug.trim() !== '' &&
    classification !== '' &&
    !create.isPending

  const handleSubmit = () => {
    if (!canSubmit || !classification) return
    create.mutate(
      {
        slug: slug.trim(),
        name: name.trim(),
        classification,
        ...(upstreamURL.trim() ? { upstream_url: upstreamURL.trim() } : {}),
        ...(upstreamCredentials.trim() ? { upstream_credentials: upstreamCredentials.trim() } : {}),
      },
      {
        onSuccess: (proj) => {
          reset()
          onClose()
          navigate({ to: '/project/$slug', params: { slug: proj.slug } })
        },
      },
    )
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Create Project</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField
            label="Name"
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            required
            autoFocus
            size="small"
            fullWidth
          />
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
          <TextField
            label="GitHub URL (optional)"
            value={upstreamURL}
            onChange={(e) => setUpstreamURL(e.target.value)}
            placeholder="https://github.com/owner/repo.git"
            size="small"
            fullWidth
            helperText="Provide to enable git proxy for srcless agents"
          />
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
          {create.isError && (
            <Typography color="error" variant="body2">
              {create.error.message}
            </Typography>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={create.isPending}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit} disabled={!canSubmit}>
          {create.isPending ? 'Creating…' : 'Create'}
        </Button>
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
