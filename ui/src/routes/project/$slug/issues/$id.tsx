import { createFileRoute } from '@tanstack/react-router'
import { IssueDetail } from '../../../../components/IssueDetail'

function IssueDetailRoute() {
  const { slug, id } = Route.useParams()
  return <IssueDetail slug={slug} issueId={id} />
}

export const Route = createFileRoute('/project/$slug/issues/$id')({
  component: IssueDetailRoute,
})
