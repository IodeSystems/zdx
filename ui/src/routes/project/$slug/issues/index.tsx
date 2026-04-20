import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { IssuesTab } from '../../../../components/IssuesTab'

const searchSchema = z.object({
  status: z.string().optional(),
})

function IssuesIndexRoute() {
  const { slug } = Route.useParams()
  const { status } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <IssuesTab
      slug={slug}
      statusFilter={status ?? null}
      onStatusFilter={(s: string | null) => navigate({ search: (prev) => ({ ...prev, status: s ?? undefined }) })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/issues/')({
  component: IssuesIndexRoute,
  validateSearch: searchSchema,
})
