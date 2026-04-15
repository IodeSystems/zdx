import { createFileRoute } from '@tanstack/react-router'
import { SpecDetail } from '../../../../components/SpecDetail'

function SpecDetailRoute() {
  const { slug, specId } = Route.useParams()
  return <SpecDetail slug={slug} specId={Number(specId)} />
}

export const Route = createFileRoute('/project/$slug/specs/$specId')({
  component: SpecDetailRoute,
})
