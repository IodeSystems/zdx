import { useState } from 'react'
import { useRouter, Link } from '@tanstack/react-router'
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
import {
  useGetConcernById,
  useLinkConcern,
  useUnlinkConcern,
  useListFeaturesForConcern,
  useListSpecsForConcern,
} from '../api'
import { MarkdownContent } from './MarkdownContent'

export function ConcernDetail({ slug, concernId }: { slug: string; concernId: number }) {
  const router = useRouter()
  const { data: concern, isLoading } = useGetConcernById(slug, concernId)
  const { data: features = [] } = useListFeaturesForConcern(slug, concernId)
  const { data: specs = [] } = useListSpecsForConcern(concernId)
  const linkConcern = useLinkConcern()
  const unlinkConcern = useUnlinkConcern()

  const [linkFeatureOpen, setLinkFeatureOpen] = useState(false)
  const [linkFeatureName, setLinkFeatureName] = useState('')
  const [linkSpecOpen, setLinkSpecOpen] = useState(false)
  const [linkSpecId, setLinkSpecId] = useState('')

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!concern) {
    return (
      <Box>
        <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
          Back
        </Button>
        <Typography color="error">Concern not found.</Typography>
      </Box>
    )
  }

  const handleLinkFeature = () => {
    if (!linkFeatureName.trim()) return
    linkConcern.mutate(
      { slug, concern_name: concern.name, feature: linkFeatureName.trim() },
      {
        onSuccess: () => {
          setLinkFeatureOpen(false)
          setLinkFeatureName('')
        },
      },
    )
  }

  const handleUnlinkFeature = (featureName: string) => {
    unlinkConcern.mutate({ slug, concern_name: concern.name, feature: featureName })
  }

  const handleLinkSpec = () => {
    const specId = parseInt(linkSpecId, 10)
    if (!specId) return
    linkConcern.mutate(
      { slug, concern_name: concern.name, spec_id: specId },
      {
        onSuccess: () => {
          setLinkSpecOpen(false)
          setLinkSpecId('')
        },
      },
    )
  }

  const handleUnlinkSpec = (specId: number) => {
    unlinkConcern.mutate({ slug, concern_name: concern.name, spec_id: specId })
  }

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        {concern.name}
      </Typography>

      {concern.description && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <MarkdownContent slug={slug}>{concern.description}</MarkdownContent>
        </Box>
      )}

      <Box sx={{ mb: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
          <Typography variant="subtitle2" color="text.secondary">
            Linked Features ({features.length})
          </Typography>
          <Button size="small" variant="outlined" onClick={() => setLinkFeatureOpen(true)}>
            Link Feature
          </Button>
        </Box>
        {features.length > 0 ? (
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
            {features.map(f => (
              <Chip
                key={f.id}
                label={f.name}
                size="small"
                variant="outlined"
                component={Link as any}
                to="/project/$slug/features/$name"
                params={{ slug, name: f.name }}
                clickable
                onDelete={() => handleUnlinkFeature(f.name)}
              />
            ))}
          </Box>
        ) : (
          <Typography variant="body2" color="text.disabled">
            No linked features.
          </Typography>
        )}
      </Box>

      <Box sx={{ mb: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
          <Typography variant="subtitle2" color="text.secondary">
            Linked Specs ({specs.length})
          </Typography>
          <Button size="small" variant="outlined" onClick={() => setLinkSpecOpen(true)}>
            Link Spec
          </Button>
        </Box>
        {specs.length > 0 ? (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {specs.map(s => (
              <Box key={s.id} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                <Chip label={`#${s.id}`} size="small" sx={{ fontFamily: 'monospace', flexShrink: 0 }} />
                <Typography variant="body2" sx={{ flexGrow: 1 }}>
                  [{s.importance}] {s.description}
                  <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 0.5 }}>
                    — {s.feature_name}
                  </Typography>
                </Typography>
                <Button
                  size="small"
                  color="error"
                  sx={{ flexShrink: 0, minWidth: 0, px: 0.5 }}
                  onClick={() => handleUnlinkSpec(s.id)}
                  disabled={unlinkConcern.isPending}
                >
                  ×
                </Button>
              </Box>
            ))}
          </Box>
        ) : (
          <Typography variant="body2" color="text.disabled">
            No linked specs.
          </Typography>
        )}
      </Box>

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 1 }}>
        Created: {new Date(concern.created_at).toLocaleString()}
      </Typography>

      <Dialog open={linkFeatureOpen} onClose={() => setLinkFeatureOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Link Feature</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            sx={{ mt: 1 }}
            label="Feature name"
            value={linkFeatureName}
            onChange={e => setLinkFeatureName(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleLinkFeature()}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setLinkFeatureOpen(false)}>Cancel</Button>
          <Button onClick={handleLinkFeature} variant="contained" disabled={linkConcern.isPending || !linkFeatureName.trim()}>
            Link
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={linkSpecOpen} onClose={() => setLinkSpecOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Link Spec</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            sx={{ mt: 1 }}
            label="Spec ID"
            type="number"
            value={linkSpecId}
            onChange={e => setLinkSpecId(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleLinkSpec()}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setLinkSpecOpen(false)}>Cancel</Button>
          <Button onClick={handleLinkSpec} variant="contained" disabled={linkConcern.isPending || !linkSpecId}>
            Link
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
