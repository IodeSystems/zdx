import { createFileRoute } from '@tanstack/react-router'
import { ErrorsTab } from '../../../../components/ErrorsTab'

function ErrorsRoute() {
  const { slug, component } = Route.useParams()
  return <ErrorsTab slug={slug} componentSlug={component} />
}

export const Route = createFileRoute('/project/$slug/$component/errors')({
  component: ErrorsRoute,
})
