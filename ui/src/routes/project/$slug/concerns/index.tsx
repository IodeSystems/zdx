import { createFileRoute } from '@tanstack/react-router'
import { ConcernsTab } from '../../../../components/ConcernsTab'

function ConcernsIndexRoute() {
  const { slug } = Route.useParams()
  return <ConcernsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/concerns/')({
  component: ConcernsIndexRoute,
})
