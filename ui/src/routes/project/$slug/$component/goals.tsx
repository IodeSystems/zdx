import { createFileRoute, Outlet } from '@tanstack/react-router'

function GoalsLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/$component/goals')({
  component: GoalsLayout,
})
