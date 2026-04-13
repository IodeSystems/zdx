import { createFileRoute } from '@tanstack/react-router'
import { ProjectDashboard } from '../../../../components/ProjectDashboard'

function ComponentDashboardRoute() {
  const { slug, component } = Route.useParams()
  return <ProjectDashboard slug={slug} componentSlug={component} />
}

export const Route = createFileRoute('/project/$slug/$component/')({
  component: ComponentDashboardRoute,
})
