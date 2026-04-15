import { createFileRoute, Outlet } from '@tanstack/react-router'

function TestsLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/tests')({
  component: TestsLayout,
})
