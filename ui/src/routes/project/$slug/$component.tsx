import { createFileRoute, Outlet } from '@tanstack/react-router'

function ComponentLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/$component')({
  component: ComponentLayout,
})
