import { createFileRoute } from '@tanstack/react-router'
import { TaskDetail } from '../../../../../components/TaskDetail'

function TaskDetailRoute() {
  const { slug, component, id } = Route.useParams()
  return <TaskDetail slug={slug} componentSlug={component} taskId={id} />
}

export const Route = createFileRoute('/project/$slug/$component/tasks/$id')({
  component: TaskDetailRoute,
})
