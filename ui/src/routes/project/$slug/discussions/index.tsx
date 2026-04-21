import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { DiscussionsTab } from '../../../../components/DiscussionsTab'

const searchSchema = z.object({
  status: z.string().optional(),
})

function DiscussionsIndexRoute() {
  const { slug } = Route.useParams()
  const { status } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <DiscussionsTab
      slug={slug}
      statusFilter={status ?? null}
      onStatusFilter={(s: string | null) => navigate({ search: (prev) => ({ ...prev, status: s ?? undefined }) })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/discussions/')({
  component: DiscussionsIndexRoute,
  validateSearch: searchSchema,
})
