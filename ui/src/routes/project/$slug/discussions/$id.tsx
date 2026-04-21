import { createFileRoute } from '@tanstack/react-router'
import { DiscussionDetail } from '../../../../components/DiscussionDetail'

function DiscussionDetailRoute() {
  const { slug, id } = Route.useParams()
  const n = Number(id)
  return <DiscussionDetail slug={slug} discussionId={Number.isFinite(n) ? n : 0} />
}

export const Route = createFileRoute('/project/$slug/discussions/$id')({
  component: DiscussionDetailRoute,
})
