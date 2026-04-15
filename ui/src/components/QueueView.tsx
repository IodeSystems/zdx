import { Link } from '@tanstack/react-router'
import {
  Box,
  Chip,
  CircularProgress,
  Paper,
  Typography,
} from '@mui/material'
import {
  useSolo,
  useBlockerQuestions,
  type SoloItem,
} from '../api'
import { MarkdownContent } from './MarkdownContent'
import { ChoiceAnswerForm } from './ChoiceAnswerForm'

const KIND_COLORS: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  triage: 'error',
  plan: 'info',
  dev: 'warning',
}

function targetLink(slug: string, targetType: string, targetId: string): { to: string; params: Record<string, string> } | null {
  if (targetType === 'issue') return { to: '/project/$slug/issues/$id', params: { slug, id: targetId } }
  if (targetType === 'task') return { to: '/project/$slug/tasks/$id', params: { slug, id: targetId } }
  if (targetType === 'feature') return { to: '/project/$slug/features/$name', params: { slug, name: targetId } }
  return null
}

export function QueueView({ slug }: { slug: string }) {
  const { data, isLoading } = useSolo(slug)
  const { data: bqData } = useBlockerQuestions(slug, 'pending')

  if (isLoading) return <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}><CircularProgress /></Box>

  const pendingQuestions = bqData?.questions ?? []
  const queue: SoloItem[] = data ?? []
  const [pick, ...rest] = queue

  if (!pick && pendingQuestions.length === 0) {
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

      {pendingQuestions.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="warning.main" sx={{ mb: 1 }}>
            Questions Blocking Progress ({pendingQuestions.length})
          </Typography>
          {pendingQuestions.map(q => (
            <Paper key={q.id} variant="outlined" sx={{ p: 2, mb: 1.5, borderColor: 'warning.main' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                {(() => {
                  const link = targetLink(slug, q.target_type, q.target_id)
                  return link ? (
                    <Chip
                      label={`${q.target_type}:${q.target_id}`}
                      size="small"
                      color="primary"
                      variant="outlined"
                      component={Link as any}
                      to={link.to}
                      params={link.params}
                      clickable
                    />
                  ) : (
                    <Chip label={`${q.target_type}:${q.target_id}`} size="small" color="primary" variant="outlined" />
                  )
                })()}
                <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.75rem' }}>
                  BQ-{q.id}
                </Typography>
              </Box>
              <MarkdownContent slug={slug} variant="body1">{q.context}</MarkdownContent>
              <ChoiceAnswerForm slug={slug} question={q} />
            </Paper>
          ))}
        </Box>
      )}

      {pick && (
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
      )}

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
