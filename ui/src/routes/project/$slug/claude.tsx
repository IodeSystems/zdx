import { createFileRoute, Outlet } from '@tanstack/react-router'

function ClaudeLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/claude')({
  component: ClaudeLayout,
})
