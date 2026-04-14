import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Chip,
  Collapse,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import {
  useChildQuestions,
  useCreateQuestion,
  useQuestions,
  useSimilarQuestions,
  type QuestionItem,
  type SimilarQuestionItem,
} from '../../../../api'
import { MarkdownContent } from '../../../../components/MarkdownContent'

function SimilarQuestionsList({
  items,
  slug,
  onProceed,
  onCancel,
}: {
  items: SimilarQuestionItem[]
  slug: string
  onProceed: () => void
  onCancel: () => void
}) {
  return (
    <Box sx={{ mt: 2, mb: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        Similar questions already asked:
      </Typography>
      {items.map((q) => (
        <Paper key={q.id} variant="outlined" sx={{ p: 1.5, mb: 1 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
            <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.75rem' }}>
              #{q.id}
            </Typography>
            <Chip label={`${Math.round(q.score * 100)}% match`} size="small" color="info" />
          </Box>
          <Typography variant="body2" sx={{ fontWeight: 500 }}>{q.question}</Typography>
          {q.answer ? (
            <Box sx={{ mt: 0.5 }}>
              <MarkdownContent slug={slug} color="text.secondary">{q.answer}</MarkdownContent>
            </Box>
          ) : (
            <Typography variant="body2" color="text.disabled" sx={{ mt: 0.5 }}>No answer yet.</Typography>
          )}
        </Paper>
      ))}
      <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
        <Button variant="contained" size="small" onClick={onProceed}>
          Ask anyway
        </Button>
        <Button variant="outlined" size="small" onClick={onCancel}>
          Cancel
        </Button>
      </Stack>
    </Box>
  )
}

function QuestionCard({ item, slug, depth = 0 }: { item: QuestionItem; slug: string; depth?: number }) {
  const questionLink = { to: '/project/$slug/questions/$id' as const, params: { slug, id: String(item.id) } }
  const [showVariation, setShowVariation] = useState(false)
  const [variationText, setVariationText] = useState('')
  const createQuestion = useCreateQuestion()
  const { data: childData } = useChildQuestions(slug, item.id)
  const children = childData?.questions ?? []

  const submitVariation = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!variationText.trim()) return
    await createQuestion.mutateAsync({
      slug,
      category: '',
      question: variationText.trim(),
      parent_question_id: item.parent_question_id ?? item.id,
    })
    setVariationText('')
    setShowVariation(false)
  }

  return (
    <Box sx={{ ml: depth > 0 ? 3 : 0 }}>
      <Paper variant="outlined" sx={{ p: 2, mb: 1.5, borderLeft: depth > 0 ? '3px solid' : undefined, borderLeftColor: depth > 0 ? 'primary.light' : undefined }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
          <Chip
            label={`#${item.id}`}
            size="small"
            variant="outlined"
            component={Link as any}
            {...questionLink}
            clickable
          />
          {item.parent_question_id && (
            <Chip label={`variation of #${item.parent_question_id}`} size="small" variant="outlined" color="info" />
          )}
        </Box>
        <Box
          component={Link as any}
          {...questionLink}
          sx={{ textDecoration: 'none', color: 'inherit', '&:hover': { opacity: 0.8 } }}
        >
          <Typography variant="body1" sx={{ fontWeight: 500 }}>{item.question}</Typography>
        </Box>
        {item.answer ? (
          <Box sx={{ mt: 1 }}>
            <MarkdownContent slug={slug} color="text.secondary">{item.answer}</MarkdownContent>
          </Box>
        ) : (
          <Typography variant="body2" color="text.disabled" sx={{ mt: 1 }}>No answer yet.</Typography>
        )}
        {!item.parent_question_id && (
          <Box sx={{ mt: 1 }}>
            <Button size="small" variant="text" onClick={() => setShowVariation(!showVariation)}>
              {showVariation ? 'Cancel' : 'Answer my variation'}
            </Button>
          </Box>
        )}
        <Collapse in={showVariation}>
          <Box component="form" onSubmit={submitVariation} sx={{ mt: 1.5 }}>
            <Stack spacing={1}>
              <TextField
                label="Your variation of this question"
                value={variationText}
                onChange={(e) => setVariationText(e.target.value)}
                multiline
                minRows={2}
                size="small"
                fullWidth
              />
              {createQuestion.isError && (
                <Alert severity="error">{(createQuestion.error as Error).message}</Alert>
              )}
              <Box>
                <Button type="submit" variant="contained" size="small" disabled={createQuestion.isPending || !variationText.trim()}>
                  {createQuestion.isPending ? 'Submitting…' : 'Ask variation'}
                </Button>
              </Box>
            </Stack>
          </Box>
        </Collapse>
      </Paper>
      {children.map(child => (
        <QuestionCard key={child.id} item={child} slug={slug} depth={depth + 1} />
      ))}
    </Box>
  )
}

function QuestionsPage() {
  const { slug } = Route.useParams()
  const { data: qData, isLoading } = useQuestions(slug)
  const data = qData?.questions
  const createQuestion = useCreateQuestion()
  const similarQuestions = useSimilarQuestions()
  const [question, setQuestion] = useState('')
  const [showSimilar, setShowSimilar] = useState(false)

  const topLevel = data?.filter(q => !q.parent_question_id) ?? []

  const checkSimilar = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!question.trim()) return
    const results = await similarQuestions.mutateAsync({ slug, text: question.trim() })
    if (results.length > 0) {
      setShowSimilar(true)
    } else {
      await doSubmit()
    }
  }

  const doSubmit = async () => {
    await createQuestion.mutateAsync({ slug, category: '', question: question.trim() })
    setQuestion('')
    setShowSimilar(false)
  }

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Questions</Typography>
      <Paper variant="outlined" sx={{ p: 2, mb: 3 }} component="form" onSubmit={checkSimilar}>
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
          {(createQuestion.isError || similarQuestions.isError) && (
            <Alert severity="error">
              {((createQuestion.error || similarQuestions.error) as Error).message}
            </Alert>
          )}
          {showSimilar && similarQuestions.data && similarQuestions.data.length > 0 && (
            <SimilarQuestionsList
              items={similarQuestions.data}
              slug={slug}
              onProceed={doSubmit}
              onCancel={() => setShowSimilar(false)}
            />
          )}
          {!showSimilar && (
            <Box>
              <Button
                type="submit"
                variant="contained"
                disabled={createQuestion.isPending || similarQuestions.isPending || !question.trim()}
              >
                {similarQuestions.isPending ? 'Checking…' : createQuestion.isPending ? 'Submitting…' : 'Ask'}
              </Button>
            </Box>
          )}
        </Stack>
      </Paper>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : topLevel.length === 0 ? (
        <Typography color="text.secondary">No questions yet.</Typography>
      ) : (
        topLevel.map(item => (
          <QuestionCard key={item.id} item={item} slug={slug} />
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/questions/')({
  component: QuestionsPage,
})
