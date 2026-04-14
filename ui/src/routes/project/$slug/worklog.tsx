import { createFileRoute } from '@tanstack/react-router'
import { WorkLogTab } from '../../../components/WorkLogTab'

function WorkLogRoute() {
  const { slug } = Route.useParams()
  return <WorkLogTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/worklog')({
  component: WorkLogRoute,
})
