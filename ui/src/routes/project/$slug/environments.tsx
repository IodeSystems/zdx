import { createFileRoute, Outlet } from '@tanstack/react-router'

function EnvironmentsLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/environments')({
  component: EnvironmentsLayout,
})
