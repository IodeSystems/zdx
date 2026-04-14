import { createFileRoute } from '@tanstack/react-router'
import { QuestionDetail } from '../../../../components/QuestionDetail'

function QuestionDetailRoute() {
  const { slug, id } = Route.useParams()
  return <QuestionDetail slug={slug} questionId={id} />
}

export const Route = createFileRoute('/project/$slug/questions/$id')({
  component: QuestionDetailRoute,
})
