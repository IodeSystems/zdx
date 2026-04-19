import { createFileRoute } from '@tanstack/react-router'
import { TodoDetail } from '../../../../components/TodoDetail'

function TodoDetailRoute() {
  const { slug, key } = Route.useParams()
  return <TodoDetail slug={slug} todoKey={decodeURIComponent(key)} />
}

export const Route = createFileRoute('/project/$slug/todos/$key')({
  component: TodoDetailRoute,
})
