import { useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import {
  useAnswerBlockerQuestion,
  type BlockerQuestionItem,
} from '../api'

const DEFER_ANSWER = 'Whatever you think is best — use your judgment.'

export function ChoiceAnswerForm({ slug, question }: { slug: string; question: BlockerQuestionItem }) {
  const answerMutation = useAnswerBlockerQuestion()
  const [selected, setSelected] = useState<number | 'other' | null>(null)
  const [customText, setCustomText] = useState('')
  const [context, setContext] = useState('')

  const choices = question.choices ?? []

  const buildAnswer = () => {
    let answer = ''
    if (selected === 'other') {
      answer = customText.trim()
    } else if (selected !== null) {
      answer = choices[selected]
    }
    if (context.trim()) {
      answer = answer ? `${answer}\n\n${context.trim()}` : context.trim()
    }
    return answer
  }

  const canSubmit = () => {
    if (answerMutation.isPending) return false
    if (selected === 'other') return customText.trim().length > 0 || context.trim().length > 0
    if (selected !== null) return true
    return context.trim().length > 0
  }

  const submit = async (answer?: string) => {
    const finalAnswer = answer ?? buildAnswer()
    if (!finalAnswer.trim()) return
    await answerMutation.mutateAsync({ slug, id: question.id, answer: finalAnswer.trim() })
    setSelected(null)
    setCustomText('')
    setContext('')
  }

  const hasChoices = choices.length > 0

  return (
    <Box sx={{ mt: 1.5 }}>
      {hasChoices && (
        <Stack spacing={1} sx={{ mb: 1.5 }}>
          {choices.map((choice, i) => (
            <Paper
              key={i}
              variant="outlined"
              onClick={() => setSelected(selected === i ? null : i)}
              sx={{
                p: 1.5,
                cursor: 'pointer',
                borderColor: selected === i ? 'primary.main' : 'divider',
                borderWidth: selected === i ? 2 : 1,
                bgcolor: selected === i ? 'primary.50' : 'transparent',
                '&:hover': { borderColor: 'primary.main', bgcolor: 'action.hover' },
                transition: 'all 0.15s ease',
              }}
            >
              <Typography variant="body2">{choice}</Typography>
            </Paper>
          ))}
          <Paper
            variant="outlined"
            onClick={() => setSelected(selected === 'other' ? null : 'other')}
            sx={{
              p: 1.5,
              cursor: 'pointer',
              borderColor: selected === 'other' ? 'primary.main' : 'divider',
              borderWidth: selected === 'other' ? 2 : 1,
              bgcolor: selected === 'other' ? 'primary.50' : 'transparent',
              '&:hover': { borderColor: 'primary.main', bgcolor: 'action.hover' },
              transition: 'all 0.15s ease',
            }}
          >
            {selected === 'other' ? (
              <TextField
                placeholder="Type your answer..."
                value={customText}
                onChange={(e) => setCustomText(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                multiline
                minRows={1}
                size="small"
                fullWidth
                autoFocus
              />
            ) : (
              <Typography variant="body2" color="text.secondary">Other...</Typography>
            )}
          </Paper>
        </Stack>
      )}

      <TextField
        label={hasChoices ? 'Additional context (optional)' : 'Answer'}
        value={context}
        onChange={(e) => setContext(e.target.value)}
        multiline
        minRows={1}
        size="small"
        fullWidth
        sx={{ mb: 1 }}
      />

      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <Button
          variant="contained"
          size="small"
          disabled={!canSubmit()}
          onClick={() => submit()}
        >
          {answerMutation.isPending ? 'Saving...' : 'Submit Answer'}
        </Button>
        <Button
          variant="outlined"
          size="small"
          disabled={answerMutation.isPending}
          onClick={() => submit(DEFER_ANSWER)}
        >
          Whatever you think is best
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
