import { createFileRoute, Outlet } from '@tanstack/react-router'

function ConcernsLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/concerns')({
  component: ConcernsLayout,
})
