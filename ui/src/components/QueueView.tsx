import { Box, Chip, CircularProgress, Typography } from '@mui/material'
import { useSolo, type SoloItem } from '../api'

const KIND_COLORS: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  triage: 'error',
  plan: 'info',
  dev: 'warning',
}

export function QueueView({ slug }: { slug: string }) {
  const { data, isLoading } = useSolo(slug)

  if (isLoading) return <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}><CircularProgress /></Box>

  const queue: SoloItem[] = data ?? []
  const [pick, ...rest] = queue

  if (!pick) {
    return (
      <Box>
        <Typography variant="h6" sx={{ mb: 1 }}>Queue</Typography>
        <Typography color="text.secondary">Nothing to do — all clear.</Typography>
      </Box>
    )
  }

  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 2 }}>Queue</Typography>

      <Box sx={{ mb: 3, p: 2, border: 1, borderColor: 'primary.main', borderRadius: 1 }}>
        <Box sx={{ display: 'flex', gap: 1, mb: 1, alignItems: 'center', flexWrap: 'wrap' }}>
          <Chip
            label={pick.kind}
            size="small"
            color={KIND_COLORS[pick.kind] || 'default'}
          />
          {pick.feature && <Chip label={pick.feature} size="small" variant="outlined" />}
          {pick.issue && <Chip label={pick.issue} size="small" variant="outlined" color="info" />}
          {pick.task && <Chip label={pick.task} size="small" variant="outlined" color="warning" />}
        </Box>
        <Typography variant="body1">{pick.summary}</Typography>
      </Box>

      {rest.length > 0 && (
        <>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Upcoming ({rest.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {rest.map((item) => (
              <Box key={item.id} sx={{ display: 'flex', gap: 1, alignItems: 'center', py: 0.5, borderBottom: 1, borderColor: 'divider' }}>
                <Chip
                  label={item.kind}
                  size="small"
                  color={KIND_COLORS[item.kind] || 'default'}
                  variant="outlined"
                />
                <Typography variant="body2" sx={{ flex: 1 }}>{item.summary}</Typography>
                {item.feature && <Chip label={item.feature} size="small" variant="outlined" sx={{ fontSize: '0.7rem' }} />}
              </Box>
            ))}
          </Box>
        </>
      )}
    </Box>
  )
}
