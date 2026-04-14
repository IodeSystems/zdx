import { createFileRoute } from '@tanstack/react-router'
import { TaskDetail } from '../../../../components/TaskDetail'

function TaskDetailRoute() {
  const { slug, id } = Route.useParams()
  return <TaskDetail slug={slug} taskId={id} />
}

export const Route = createFileRoute('/project/$slug/tasks/$id')({
  component: TaskDetailRoute,
})
