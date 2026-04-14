import { createFileRoute, Outlet } from '@tanstack/react-router'
import { ComponentProvider } from '../../components/ComponentContext'

function ProjectLayout() {
  const { slug } = Route.useParams()
  return (
    <ComponentProvider slug={slug}>
      <Outlet />
    </ComponentProvider>
  )
}

export const Route = createFileRoute('/project/$slug')({
  component: ProjectLayout,
})
