import { createFileRoute } from '@tanstack/react-router'
import { WorkLogTab } from '../../../../components/WorkLogTab'

function WorkLogRoute() {
  const { slug, component } = Route.useParams()
  return <WorkLogTab slug={slug} componentSlug={component} />
}

export const Route = createFileRoute('/project/$slug/$component/worklog')({
  component: WorkLogRoute,
})
