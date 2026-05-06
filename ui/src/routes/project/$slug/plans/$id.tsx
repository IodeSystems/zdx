import { createFileRoute } from '@tanstack/react-router'
import { PlanDetail } from '../../../../components/PlanDetail'

function PlanDetailRoute() {
  const { slug, id } = Route.useParams()
  const n = Number(id)
  return <PlanDetail slug={slug} planId={Number.isFinite(n) ? n : 0} />
}

export const Route = createFileRoute('/project/$slug/plans/$id')({
  component: PlanDetailRoute,
})
