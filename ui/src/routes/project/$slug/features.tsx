import { createFileRoute, Outlet } from '@tanstack/react-router'

function FeaturesLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/features')({
  component: FeaturesLayout,
})
