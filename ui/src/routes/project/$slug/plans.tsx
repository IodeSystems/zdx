import { createFileRoute, Outlet } from '@tanstack/react-router'

function PlansLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/plans')({
  component: PlansLayout,
})
