import { createFileRoute, Outlet } from '@tanstack/react-router'

function BranchesLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/branches')({
  component: BranchesLayout,
})
