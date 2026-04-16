import { createFileRoute } from '@tanstack/react-router'
import { FocusesTab } from '../../../../components/FocusesTab'

function FocusesIndexRoute() {
  const { slug } = Route.useParams()
  return <FocusesTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/focuses/')({
  component: FocusesIndexRoute,
})
