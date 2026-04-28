import { useState } from 'react'
import { useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useGetPattern, useDeletePattern, useUpdatePattern, useCreateIssue } from '../api'
import { MarkdownContent } from './MarkdownContent'
import { CommentsAndRevisions } from './CommentsAndRevisions'

export function PatternDetail({ slug, patternId }: { slug: string; patternId: number }) {
  const router = useRouter()
  const { data: pattern, isLoading } = useGetPattern(slug, patternId)
  const deletePattern = useDeletePattern()
  const updatePattern = useUpdatePattern()
  const createIssue = useCreateIssue()

  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [issueOpen, setIssueOpen] = useState(false)
  const [issueContext, setIssueContext] = useState('')

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!pattern) {
    return (
      <Box>
        <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
          Back
        </Button>
        <Typography color="error">Pattern not found.</Typography>
      </Box>
    )
  }

  const refs = Array.isArray(pattern.code_refs) ? pattern.code_refs : []

  const handleDelete = () => {
    deletePattern.mutate(
      { slug, id: patternId },
      { onSuccess: () => router.history.go(-1) },
    )
  }

  const handleEditStart = () => {
    setEditName(pattern.name)
    setEditDescription(pattern.description)
    setEditing(true)
  }

  const handleEditSave = () => {
    updatePattern.mutate(
      { slug, id: patternId, name: editName, description: editDescription, code_refs: refs },
      { onSuccess: () => setEditing(false) },
    )
  }

  const handleFileIssue = () => {
    setIssueContext(`PT-${pattern.id}: ${pattern.name}\n\n`)
    setIssueOpen(true)
  }

  const handleSubmitIssue = () => {
    createIssue.mutate(
      { slug, context: issueContext.trim() || undefined, auto_ready: true },
      {
        onSuccess: () => {
          setIssueOpen(false)
          setIssueContext('')
        },
      },
    )
  }

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back
      </Button>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
        {editing ? (
          <TextField
            value={editName}
            onChange={e => setEditName(e.target.value)}
            size="small"
            sx={{ flexGrow: 1, mr: 1 }}
            label="Name"
          />
        ) : (
          <Typography variant="h5">
            PT-{pattern.id}: {pattern.name}
          </Typography>
        )}
        <Box sx={{ display: 'flex', gap: 1, flexShrink: 0 }}>
          {editing ? (
            <>
              <Button size="small" variant="contained" onClick={handleEditSave} disabled={updatePattern.isPending}>
                Save
              </Button>
              <Button size="small" onClick={() => setEditing(false)}>Cancel</Button>
            </>
          ) : (
            <>
              <Button size="small" variant="outlined" onClick={handleEditStart}>Edit</Button>
              <Button size="small" color="error" variant="outlined" onClick={handleDelete} disabled={deletePattern.isPending}>
                Delete
              </Button>
            </>
          )}
        </Box>
      </Box>

      {editing ? (
        <TextField
          value={editDescription}
          onChange={e => setEditDescription(e.target.value)}
          multiline
          minRows={4}
          fullWidth
          sx={{ mb: 3 }}
          label="Description"
        />
      ) : pattern.description ? (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <MarkdownContent slug={slug}>{pattern.description}</MarkdownContent>
        </Box>
      ) : null}

      {refs.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Code References ({refs.length})
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
            {refs.map((r, i) => (
              <Chip key={i} label={r.path} size="small" variant="outlined" sx={{ fontFamily: 'monospace' }} />
            ))}
          </Box>
        </Box>
      )}

      <Box sx={{ mb: 2 }}>
        <Button size="small" variant="outlined" onClick={handleFileIssue}>
          File issue against pattern
        </Button>
      </Box>

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 1, mb: 3 }}>
        Created: {new Date(pattern.created_at).toLocaleString()}
        {' | '}
        Updated: {new Date(pattern.updated_at).toLocaleString()}
      </Typography>

      <CommentsAndRevisions slug={slug} targetType="pattern" targetId={String(patternId)} />

      <Dialog open={issueOpen} onClose={() => setIssueOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>File issue against pattern</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            multiline
            rows={5}
            sx={{ mt: 1 }}
            value={issueContext}
            onChange={e => setIssueContext(e.target.value)}
            placeholder="Describe what needs to be done for this pattern..."
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIssueOpen(false)}>Cancel</Button>
          <Button onClick={handleSubmitIssue} variant="contained" disabled={createIssue.isPending}>
            Create issue
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
