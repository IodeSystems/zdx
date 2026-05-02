import { createFileRoute } from '@tanstack/react-router'
import { BlockedWorkPanel } from '../../../components/BlockedWorkPanel'

function BlockedWorkRoute() {
  const { slug } = Route.useParams()
  return <BlockedWorkPanel slug={slug} />
}

export const Route = createFileRoute('/project/$slug/blocked-work')({
  component: BlockedWorkRoute,
})
