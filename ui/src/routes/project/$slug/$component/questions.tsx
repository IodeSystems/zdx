import { createFileRoute } from '@tanstack/react-router'
import {
  Box,
  Chip,
  CircularProgress,
  Paper,
  Typography,
} from '@mui/material'
import { useQuestions } from '../../../../api'

function QuestionsPage() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useQuestions(slug)

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />
  if (!data || data.length === 0) return <Typography color="text.secondary">No questions.</Typography>

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Questions</Typography>
      {data.map(item => (
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
      ))}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/$component/questions')({
  component: QuestionsPage,
})
