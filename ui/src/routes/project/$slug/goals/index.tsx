import { createFileRoute } from '@tanstack/react-router'
import { GoalsTab } from '../../../../components/GoalsTab'

function GoalsIndexRoute() {
  const { slug } = Route.useParams()
  return <GoalsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/goals/')({
  component: GoalsIndexRoute,
})
