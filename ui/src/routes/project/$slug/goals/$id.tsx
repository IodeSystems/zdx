import { createFileRoute } from '@tanstack/react-router'
import { GoalDetail } from '../../../../components/GoalDetail'

function GoalDetailRoute() {
  const { slug, id } = Route.useParams()
  const n = Number(id)
  return <GoalDetail slug={slug} goalId={Number.isFinite(n) ? n : 0} />
}

export const Route = createFileRoute('/project/$slug/goals/$id')({
  component: GoalDetailRoute,
})
