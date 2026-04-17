import { createFileRoute, Outlet } from '@tanstack/react-router'

function DemosLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/demos')({
  component: DemosLayout,
})
