import {
  Box,
  Chip,
  Typography,
} from '@mui/material'
import {
  useBlockerQuestionsByTarget,
} from '../api'
import { MarkdownContent } from './MarkdownContent'
import { ChoiceAnswerForm } from './ChoiceAnswerForm'

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
          <MarkdownContent slug={slug}>{q.context}</MarkdownContent>
          {q.answer ? (
            <Box sx={{ mt: 1, p: 1, bgcolor: 'action.hover', borderRadius: 1 }}>
              <MarkdownContent slug={slug}>{q.answer}</MarkdownContent>
              {q.answered_by && (
                <Typography variant="caption" color="text.secondary">
                  — {q.answered_by}
                </Typography>
              )}
            </Box>
          ) : (
            <ChoiceAnswerForm slug={slug} question={q} />
          )}
        </Box>
      ))}
    </Box>
  )
}
