import { createFileRoute } from '@tanstack/react-router'
import { ClaudeSessionsTab } from '../../../../components/ClaudeSessionsTab'

function ClaudeIndexRoute() {
  const { slug } = Route.useParams()
  return <ClaudeSessionsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/agents/')({
  component: ClaudeIndexRoute,
})
