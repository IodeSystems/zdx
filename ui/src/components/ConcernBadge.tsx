import { Chip } from '@mui/material'
import { Link } from '@tanstack/react-router'
import type { ConcernItem } from '../api'

export function ConcernBadge({ slug, concern }: { slug: string; concern: ConcernItem }) {
  return (
    <Chip
      label={concern.name}
      size="small"
      variant="outlined"
      sx={{ height: 18, fontSize: '0.7rem' }}
      component={Link as any}
      to="/project/$slug/concerns/$id"
      params={{ slug, id: String(concern.id) }}
      clickable
    />
  )
}
