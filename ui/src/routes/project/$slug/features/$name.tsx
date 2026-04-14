import { createFileRoute } from '@tanstack/react-router'
import { FeatureDetail } from '../../../../components/FeatureDetail'

function FeatureDetailRoute() {
  const { slug, name } = Route.useParams()
  return <FeatureDetail slug={slug} name={name} />
}

export const Route = createFileRoute('/project/$slug/features/$name')({
  component: FeatureDetailRoute,
})
