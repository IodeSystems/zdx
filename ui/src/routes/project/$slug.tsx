import { createFileRoute, Outlet } from '@tanstack/react-router'

function ProjectLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug')({
  component: ProjectLayout,
})
