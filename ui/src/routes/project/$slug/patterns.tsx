import { createFileRoute, Outlet } from '@tanstack/react-router'

function PatternsLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/patterns')({
  component: PatternsLayout,
})
