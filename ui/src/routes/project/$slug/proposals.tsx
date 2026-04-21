import { createFileRoute, Outlet } from '@tanstack/react-router'

function ProposalsLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/proposals')({
  component: ProposalsLayout,
})
