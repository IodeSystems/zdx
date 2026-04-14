import { createFileRoute } from '@tanstack/react-router'
import { ClaudeSessionsTab } from '../../../../components/ClaudeSessionsTab'

function ClaudeRoute() {
  const { slug, component } = Route.useParams()
  return <ClaudeSessionsTab slug={slug} componentSlug={component} />
}

export const Route = createFileRoute('/project/$slug/$component/claude')({
  component: ClaudeRoute,
})
