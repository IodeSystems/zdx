import { createFileRoute } from '@tanstack/react-router'
import { PatternsTab } from '../../../../components/PatternsTab'

function PatternsIndexRoute() {
  const { slug } = Route.useParams()
  return <PatternsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/patterns/')({
  component: PatternsIndexRoute,
})
