import { useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { useCreateQuestion, useQuestions } from '../../../../api'

function QuestionsPage() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useQuestions(slug)
  const createQuestion = useCreateQuestion()
  const [question, setQuestion] = useState('')
  const [category, setCategory] = useState('')

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!question.trim()) return
    await createQuestion.mutateAsync({ slug, category: category.trim(), question: question.trim() })
    setQuestion('')
    setCategory('')
  }

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Questions</Typography>
      <Paper variant="outlined" sx={{ p: 2, mb: 3 }} component="form" onSubmit={submit}>
        <Stack spacing={1.5}>
          <TextField
            label="Question"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            multiline
            minRows={2}
            required
            size="small"
          />
          <TextField
            label="Category (optional)"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            size="small"
          />
          {createQuestion.isError && (
            <Alert severity="error">{(createQuestion.error as Error).message}</Alert>
          )}
          <Box>
            <Button type="submit" variant="contained" disabled={createQuestion.isPending || !question.trim()}>
              {createQuestion.isPending ? 'Submitting…' : 'Ask'}
            </Button>
          </Box>
        </Stack>
      </Paper>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : !data || data.length === 0 ? (
        <Typography color="text.secondary">No questions yet.</Typography>
      ) : (
        data.map(item => (
          <Paper key={item.id} variant="outlined" sx={{ p: 2, mb: 2 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
              {item.category && <Chip label={item.category} size="small" />}
              <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.75rem' }}>
                #{item.id}
              </Typography>
            </Box>
            <Typography variant="body1" sx={{ fontWeight: 500 }}>{item.question}</Typography>
            {item.answer ? (
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1, whiteSpace: 'pre-wrap' }}>
                {item.answer}
              </Typography>
            ) : (
              <Typography variant="body2" color="text.disabled" sx={{ mt: 1 }}>No answer yet.</Typography>
            )}
          </Paper>
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/$component/questions')({
  component: QuestionsPage,
})
