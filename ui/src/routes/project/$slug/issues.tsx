import { createFileRoute, Outlet } from '@tanstack/react-router'

function IssuesLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/issues')({
  component: IssuesLayout,
})
