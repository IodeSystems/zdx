import { useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useGetPattern, useDeletePattern } from '../api'
import { MarkdownContent } from './MarkdownContent'

export function PatternDetail({ slug, patternId }: { slug: string; patternId: number }) {
  const router = useRouter()
  const { data: pattern, isLoading } = useGetPattern(slug, patternId)
  const deletePattern = useDeletePattern()

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

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back
      </Button>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
        <Typography variant="h5">
          PT-{pattern.id}: {pattern.name}
        </Typography>
        <Button size="small" color="error" variant="outlined" onClick={handleDelete} disabled={deletePattern.isPending}>
          Delete
        </Button>
      </Box>

      {pattern.description && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <MarkdownContent slug={slug}>{pattern.description}</MarkdownContent>
        </Box>
      )}

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

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3 }}>
        Created: {new Date(pattern.created_at).toLocaleString()}
        {' | '}
        Updated: {new Date(pattern.updated_at).toLocaleString()}
      </Typography>
    </Box>
  )
}
