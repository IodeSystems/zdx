import { createFileRoute } from '@tanstack/react-router'
import { QueueView } from '../../../components/QueueView'

function QueueRoute() {
  const { slug } = Route.useParams()
  return <QueueView slug={slug} />
}

export const Route = createFileRoute('/project/$slug/queue')({
  component: QueueRoute,
})
