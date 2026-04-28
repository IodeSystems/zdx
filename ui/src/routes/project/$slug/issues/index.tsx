import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { IssuesTab } from '../../../../components/IssuesTab'

const searchSchema = z.object({
  status: z.string().optional(),
  page: z.number().int().min(1).optional(),
})

function IssuesIndexRoute() {
  const { slug } = Route.useParams()
  const { status, page } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <IssuesTab
      slug={slug}
      statusFilter={status ?? null}
      onStatusFilter={(s: string | null) => navigate({ search: (prev) => ({ ...prev, status: s ?? undefined }) })}
      page={page ?? 1}
      onPageChange={(p: number) => navigate({ search: (prev) => ({ ...prev, page: p === 1 ? undefined : p }) })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/issues/')({
  component: IssuesIndexRoute,
  validateSearch: searchSchema,
})
