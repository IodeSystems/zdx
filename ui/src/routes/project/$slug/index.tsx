import { createFileRoute } from '@tanstack/react-router'
import { ProjectDashboard } from '../../../components/ProjectDashboard'

function ProjectDashboardRoute() {
  const { slug } = Route.useParams()
  return <ProjectDashboard slug={slug} />
}

export const Route = createFileRoute('/project/$slug/')({
  component: ProjectDashboardRoute,
})
