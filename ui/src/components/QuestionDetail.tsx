import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Paper,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useEffect } from 'react'
import {
  useQuestion,
  useChildQuestions,
  type QuestionItem,
} from '../api'
import { CommentsAndRevisions } from './CommentsAndRevisions'

function ChildQuestionCard({ item, slug }: { item: QuestionItem; slug: string }) {
  const { data: childData } = useChildQuestions(slug, item.id)
  const children = childData?.questions ?? []

  return (
    <Box sx={{ ml: 3 }}>
      <Paper
        variant="outlined"
        sx={{ p: 2, mb: 1.5, borderLeft: '3px solid', borderLeftColor: 'primary.light' }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
          <Chip
            label={`#${item.id}`}
            size="small"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/questions/$id"
            params={{ slug, id: String(item.id) }}
            clickable
          />
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
      {children.map(child => (
        <ChildQuestionCard key={child.id} item={child} slug={slug} />
      ))}
    </Box>
  )
}

export function QuestionDetail({ slug, questionId }: { slug: string; questionId: string }) {
  const id = Number(questionId)
  const { data: question, isLoading } = useQuestion(slug, id)
  const { data: childData } = useChildQuestions(slug, id)
  const children = childData?.questions ?? []
  const router = useRouter()

  useEffect(() => {
    if (!question) return
    const title = question.question.slice(0, 60) + (question.question.length > 60 ? '…' : '')
    document.title = `Q#${questionId}: ${title} | zdx`
    return () => { document.title = 'zdx' }
  }, [questionId, question])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (!question) {
    return (
      <Box>
        <Button
          startIcon={<ArrowBackIcon />}
          size="small"
          sx={{ mb: 2 }}
          onClick={() => router.history.go(-1)}
        >
          Back
        </Button>
        <Typography color="error">Question {questionId} not found.</Typography>
      </Box>
    )
  }

  return (
    <Box>
      <Button
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
        onClick={() => router.history.go(-1)}
      >
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        Q#{questionId}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        {question.category && (
          <Chip label={question.category} size="small" variant="outlined" />
        )}
        {question.parent_question_id && (
          <Chip
            label={`variation of #${question.parent_question_id}`}
            size="small"
            variant="outlined"
            color="info"
            component={Link as any}
            to="/project/$slug/questions/$id"
            params={{ slug, id: String(question.parent_question_id) }}
            clickable
          />
        )}
        <Chip
          label={question.answer ? 'answered' : 'unanswered'}
          size="small"
          color={question.answer ? 'success' : 'warning'}
          variant="outlined"
        />
      </Box>

      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Question
        </Typography>
        <Typography variant="body1" sx={{ whiteSpace: 'pre-wrap' }}>
          {question.question}
        </Typography>
      </Box>

      {question.answer && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Answer
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
            {question.answer}
          </Typography>
        </Box>
      )}

      {children.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Variations ({children.length})
          </Typography>
          {children.map(child => (
            <ChildQuestionCard key={child.id} item={child} slug={slug} />
          ))}
        </Box>
      )}

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 3 }}>
        Created: {question.created_at}
        {question.updated_at !== question.created_at && ` · Updated: ${question.updated_at}`}
      </Typography>

      <CommentsAndRevisions slug={slug} targetType="question" targetId={questionId} />
    </Box>
  )
}
