import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { SessionDetail } from '../../../../components/ClaudeSessionsTab'

function ClaudeSessionRoute() {
  const { slug, sessionId } = Route.useParams()
  const navigate = useNavigate()
  return (
    <SessionDetail
      slug={slug}
      sessionId={Number(sessionId)}
      onBack={() => navigate({ to: '/project/$slug/claude', params: { slug } })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/claude/$sessionId')({
  component: ClaudeSessionRoute,
})
