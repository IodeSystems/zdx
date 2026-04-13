import { createFileRoute } from '@tanstack/react-router'
import { IssueDetail } from '../../../../../components/IssueDetail'

function IssueDetailRoute() {
  const { slug, component, id } = Route.useParams()
  return <IssueDetail slug={slug} componentSlug={component} issueId={id} />
}

export const Route = createFileRoute('/project/$slug/$component/issues/$id')({
  component: IssueDetailRoute,
})
