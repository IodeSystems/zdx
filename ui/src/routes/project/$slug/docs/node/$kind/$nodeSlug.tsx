import { createFileRoute } from '@tanstack/react-router'
import { AtlasNodeDetail } from '../../../../../../components/AtlasNodeDetail'

function AtlasNodeDetailRoute() {
  const { slug, kind, nodeSlug } = Route.useParams()
  return <AtlasNodeDetail slug={slug} kind={kind} nodeSlug={nodeSlug} />
}

export const Route = createFileRoute('/project/$slug/docs/node/$kind/$nodeSlug')({
  component: AtlasNodeDetailRoute,
})
