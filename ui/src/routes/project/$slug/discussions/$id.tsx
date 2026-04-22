import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { DiscussionDetail } from '../../../../components/DiscussionDetail'

const searchSchema = z.object({
  initialMessage: z.string().optional(),
})

function DiscussionDetailRoute() {
  const { slug, id } = Route.useParams()
  const { initialMessage } = Route.useSearch()
  const n = Number(id)
  return (
    <DiscussionDetail
      slug={slug}
      discussionId={Number.isFinite(n) ? n : 0}
      initialMessage={initialMessage}
    />
  )
}

export const Route = createFileRoute('/project/$slug/discussions/$id')({
  component: DiscussionDetailRoute,
  validateSearch: searchSchema,
})
