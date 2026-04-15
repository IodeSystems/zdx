import { createFileRoute } from '@tanstack/react-router'
import { JournalDetail } from '../../../../components/JournalDetail'

function JournalDetailRoute() {
  const { slug, id } = Route.useParams()
  return <JournalDetail slug={slug} entryId={id} />
}

export const Route = createFileRoute('/project/$slug/journal/$id')({
  component: JournalDetailRoute,
})
