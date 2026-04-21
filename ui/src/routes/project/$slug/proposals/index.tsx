import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { ProposalsTab } from '../../../../components/ProposalsTab'

const searchSchema = z.object({
  status: z.string().optional(),
})

function ProposalsIndexRoute() {
  const { slug } = Route.useParams()
  const { status } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <ProposalsTab
      slug={slug}
      statusFilter={status ?? null}
      onStatusFilter={(s: string | null) => navigate({ search: (prev) => ({ ...prev, status: s ?? undefined }) })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/proposals/')({
  component: ProposalsIndexRoute,
  validateSearch: searchSchema,
})
