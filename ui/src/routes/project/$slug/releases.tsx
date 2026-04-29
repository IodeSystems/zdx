import { createFileRoute, Outlet } from '@tanstack/react-router'

function ReleasesLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/releases')({
  component: ReleasesLayout,
})
