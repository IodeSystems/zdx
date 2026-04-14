import { useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import {
  useBlockerQuestionsByTarget,
  useAnswerBlockerQuestion,
  type BlockerQuestionItem,
} from '../api'

function InlineAnswerForm({ slug, question }: { slug: string; question: BlockerQuestionItem }) {
  const answerMutation = useAnswerBlockerQuestion()
  const [answer, setAnswer] = useState('')

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!answer.trim()) return
    await answerMutation.mutateAsync({ slug, id: question.id, answer: answer.trim() })
    setAnswer('')
  }

  return (
    <Box component="form" onSubmit={submit} sx={{ mt: 1 }}>
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

export function BlockerQuestionsSection({
  slug,
  targetType,
  targetId,
}: {
  slug: string
  targetType: string
  targetId: string
}) {
  const { data } = useBlockerQuestionsByTarget(slug, targetType, targetId)
  const questions = data?.questions ?? []

  if (questions.length === 0) return null

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        Blocker Questions ({questions.length})
      </Typography>
      {questions.map(q => (
        <Box key={q.id} sx={{ borderLeft: 2, borderColor: q.status === 'pending' ? 'warning.main' : 'success.main', pl: 1.5, mb: 1.5 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
            <Typography variant="caption" color="text.secondary">BQ-{q.id}</Typography>
            <Chip
              label={q.status}
              size="small"
              color={q.status === 'pending' ? 'warning' : 'success'}
            />
          </Box>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
            {q.context}
          </Typography>
          {q.choices.length > 0 && (
            <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', mt: 0.5 }}>
              {q.choices.map((c, i) => (
                <Chip key={i} label={c} size="small" variant="outlined" />
              ))}
            </Box>
          )}
          {q.answer ? (
            <Box sx={{ mt: 1, p: 1, bgcolor: 'action.hover', borderRadius: 1 }}>
              <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
                {q.answer}
              </Typography>
              {q.answered_by && (
                <Typography variant="caption" color="text.secondary">
                  — {q.answered_by}
                </Typography>
              )}
            </Box>
          ) : (
            <InlineAnswerForm slug={slug} question={q} />
          )}
        </Box>
      ))}
    </Box>
  )
}
