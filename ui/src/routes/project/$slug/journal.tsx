import { createFileRoute } from '@tanstack/react-router'
import { JournalTab } from '../../../components/JournalTab'

function JournalRoute() {
  const { slug } = Route.useParams()
  return <JournalTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/journal')({
  component: JournalRoute,
})
