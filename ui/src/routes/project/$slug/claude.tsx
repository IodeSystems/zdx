import { createFileRoute } from '@tanstack/react-router'
import { ClaudeSessionsTab } from '../../../components/ClaudeSessionsTab'

function ClaudeRoute() {
  const { slug } = Route.useParams()
  return <ClaudeSessionsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/claude')({
  component: ClaudeRoute,
})
