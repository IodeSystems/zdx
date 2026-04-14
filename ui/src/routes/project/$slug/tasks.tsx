import { createFileRoute, Outlet } from '@tanstack/react-router'

function TasksLayout() {
  return <Outlet />
}

export const Route = createFileRoute('/project/$slug/tasks')({
  component: TasksLayout,
})
