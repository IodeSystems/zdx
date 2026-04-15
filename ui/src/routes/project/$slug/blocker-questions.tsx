import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Chip,
  CircularProgress,
  Paper,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material'
import {
  useBlockerQuestions,
} from '../../../api'
import { MarkdownContent } from '../../../components/MarkdownContent'
import { ChoiceAnswerForm } from '../../../components/ChoiceAnswerForm'

function targetLink(slug: string, targetType: string, targetId: string): { to: string; params: Record<string, string> } | null {
  if (targetType === 'issue') return { to: '/project/$slug/issues/$id', params: { slug, id: targetId } }
  if (targetType === 'task') return { to: '/project/$slug/tasks/$id', params: { slug, id: targetId } }
  if (targetType === 'feature') return { to: '/project/$slug/features/$name', params: { slug, name: targetId } }
  return null
}

function BlockerQuestionsPage() {
  const { slug } = Route.useParams()
  const [filter, setFilter] = useState<string>('pending')
  const { data: bqData, isLoading } = useBlockerQuestions(slug, filter || undefined)
  const data = bqData?.questions

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="h6" sx={{ fontWeight: 600 }}>Blocker Questions</Typography>
        <ToggleButtonGroup
          value={filter}
          exclusive
          onChange={(_, v) => { if (v !== null) setFilter(v) }}
          size="small"
        >
          <ToggleButton value="pending">Pending</ToggleButton>
          <ToggleButton value="">All</ToggleButton>
        </ToggleButtonGroup>
      </Box>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : !data || data.length === 0 ? (
        <Typography color="text.secondary">No blocker questions.</Typography>
      ) : (
        data.map(item => (
          <Paper key={item.id} variant="outlined" sx={{ p: 2, mb: 2 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
              {(() => {
                const link = targetLink(slug, item.target_type, item.target_id)
                return link ? (
                  <Chip
                    label={`${item.target_type}:${item.target_id}`}
                    size="small"
                    color="primary"
                    variant="outlined"
                    component={Link as any}
                    to={link.to}
                    params={link.params}
                    clickable
                  />
                ) : (
                  <Chip
                    label={`${item.target_type}:${item.target_id}`}
                    size="small"
                    color="primary"
                    variant="outlined"
                  />
                )
              })()}
              <Chip
                label={item.status}
                size="small"
                color={item.status === 'pending' ? 'warning' : 'success'}
              />
              <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.75rem' }}>
                BQ-{item.id}
              </Typography>
            </Box>
            <MarkdownContent slug={slug} variant="body1">{item.context}</MarkdownContent>
            {item.answer ? (
              <Box sx={{ mt: 1.5, p: 1.5, bgcolor: 'action.hover', borderRadius: 1 }}>
                <MarkdownContent slug={slug}>{item.answer}</MarkdownContent>
                {item.answered_by && (
                  <Typography variant="caption" color="text.secondary">
                    — {item.answered_by}
                  </Typography>
                )}
              </Box>
            ) : (
              <ChoiceAnswerForm slug={slug} question={item} />
            )}
          </Paper>
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/blocker-questions')({
  component: BlockerQuestionsPage,
})
