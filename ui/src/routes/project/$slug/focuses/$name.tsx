import { createFileRoute } from '@tanstack/react-router'
import { FocusDetail } from '../../../../components/FocusDetail'

function FocusDetailRoute() {
  const { slug, name } = Route.useParams()
  return <FocusDetail slug={slug} name={decodeURIComponent(name)} />
}

export const Route = createFileRoute('/project/$slug/focuses/$name')({
  component: FocusDetailRoute,
})
