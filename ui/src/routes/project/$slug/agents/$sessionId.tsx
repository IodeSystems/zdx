import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { SessionDetail } from '../../../../components/ClaudeSessionsTab'

function ClaudeSessionRoute() {
  const { slug, sessionId } = Route.useParams()
  const navigate = useNavigate()
  return (
    <SessionDetail
      slug={slug}
      sessionId={Number(sessionId)}
      onBack={() => navigate({ to: '/project/$slug/agents', params: { slug } })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/agents/$sessionId')({
  component: ClaudeSessionRoute,
})
