import { createFileRoute } from '@tanstack/react-router'
import { PatternDetail } from '../../../../components/PatternDetail'

function PatternDetailRoute() {
  const { slug, id } = Route.useParams()
  return <PatternDetail slug={slug} patternId={Number(id)} />
}

export const Route = createFileRoute('/project/$slug/patterns/$id')({
  component: PatternDetailRoute,
})
