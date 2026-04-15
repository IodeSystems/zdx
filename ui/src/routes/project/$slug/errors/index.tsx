import { createFileRoute } from '@tanstack/react-router'
import { ErrorsTab } from '../../../../components/ErrorsTab'

function ErrorsIndexRoute() {
  const { slug } = Route.useParams()
  return <ErrorsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/errors/')({
  component: ErrorsIndexRoute,
})
