import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { TasksTab } from '../../../../../components/TasksTab'

const searchSchema = z.object({
  status: z.string().optional(),
  page: z.number().optional(),
  search: z.string().optional(),
})

function TasksIndexRoute() {
  const { slug, component } = Route.useParams()
  const { status, page, search } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <TasksTab
      slug={slug}
      componentSlug={component}
      statusFilter={status ?? null}
      page={page ?? 1}
      search={search ?? ''}
      onStatusFilter={(s: string | null) => navigate({ search: (prev) => ({ ...prev, status: s ?? undefined, page: undefined }) })}
      onPageChange={(p: number) => navigate({ search: (prev) => ({ ...prev, page: p === 1 ? undefined : p }) })}
      onSearch={(s: string) => navigate({ search: (prev) => ({ ...prev, search: s || undefined, page: undefined }) })}
    />
  )
}

export const Route = createFileRoute('/project/$slug/$component/tasks/')({
  component: TasksIndexRoute,
  validateSearch: searchSchema,
})
