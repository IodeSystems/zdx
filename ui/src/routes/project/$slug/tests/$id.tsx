import { createFileRoute } from '@tanstack/react-router'
import { TestDetail } from '../../../../components/TestDetail'

function TestDetailRoute() {
  const { slug, id } = Route.useParams()
  return <TestDetail slug={slug} testId={Number(id)} />
}

export const Route = createFileRoute('/project/$slug/tests/$id')({
  component: TestDetailRoute,
})
