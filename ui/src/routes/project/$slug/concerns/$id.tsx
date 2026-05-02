import { createFileRoute } from '@tanstack/react-router'
import { ConcernDetail } from '../../../../components/ConcernDetail'

function ConcernDetailRoute() {
  const { slug, id } = Route.useParams()
  return <ConcernDetail slug={slug} concernId={Number(id)} />
}

export const Route = createFileRoute('/project/$slug/concerns/$id')({
  component: ConcernDetailRoute,
})
