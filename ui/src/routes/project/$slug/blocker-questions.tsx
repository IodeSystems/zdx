import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Paper,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material'
import {
  useBlockerQuestions,
  useAnswerBlockerQuestion,
  type BlockerQuestionItem,
} from '../../../api'

function AnswerForm({ slug, question }: { slug: string; question: BlockerQuestionItem }) {
  const answerMutation = useAnswerBlockerQuestion()
  const [answer, setAnswer] = useState('')

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!answer.trim()) return
    await answerMutation.mutateAsync({ slug, id: question.id, answer: answer.trim() })
    setAnswer('')
  }

  return (
    <Box component="form" onSubmit={submit} sx={{ mt: 1.5 }}>
      <Stack spacing={1} direction="row" sx={{ alignItems: 'flex-start' }}>
        <TextField
          label="Answer"
          value={answer}
          onChange={(e) => setAnswer(e.target.value)}
          multiline
          minRows={1}
          size="small"
          sx={{ flex: 1 }}
        />
        <Button
          type="submit"
          variant="contained"
          size="small"
          disabled={answerMutation.isPending || !answer.trim()}
        >
          {answerMutation.isPending ? 'Saving...' : 'Answer'}
        </Button>
      </Stack>
      {answerMutation.isError && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {(answerMutation.error as Error).message}
        </Alert>
      )}
    </Box>
  )
}

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
            <Typography variant="body1" sx={{ fontWeight: 500, whiteSpace: 'pre-wrap' }}>
              {item.context}
            </Typography>
            {item.choices.length > 0 && (
              <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', mt: 1 }}>
                {item.choices.map((c, i) => (
                  <Chip key={i} label={c} size="small" variant="outlined" />
                ))}
              </Box>
            )}
            {item.answer ? (
              <Box sx={{ mt: 1.5, p: 1.5, bgcolor: 'action.hover', borderRadius: 1 }}>
                <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
                  {item.answer}
                </Typography>
                {item.answered_by && (
                  <Typography variant="caption" color="text.secondary">
                    — {item.answered_by}
                  </Typography>
                )}
              </Box>
            ) : (
              <AnswerForm slug={slug} question={item} />
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
