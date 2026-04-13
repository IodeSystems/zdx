import { createFileRoute } from '@tanstack/react-router'
import { FeatureDetail } from '../../../../../components/FeatureDetail'

function FeatureDetailRoute() {
  const { slug, component, name } = Route.useParams()
  return <FeatureDetail slug={slug} componentSlug={component} name={name} />
}

export const Route = createFileRoute('/project/$slug/$component/features/$name')({
  component: FeatureDetailRoute,
})
