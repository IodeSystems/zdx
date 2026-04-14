import { createFileRoute } from '@tanstack/react-router'
import { ErrorsTab } from '../../../components/ErrorsTab'

function ErrorsRoute() {
  const { slug } = Route.useParams()
  return <ErrorsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/errors')({
  component: ErrorsRoute,
})
