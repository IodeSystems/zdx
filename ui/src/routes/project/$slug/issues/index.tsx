import { createFileRoute } from '@tanstack/react-router'
import { IssuesTab } from '../../../../components/IssuesTab'

function IssuesIndexRoute() {
  const { slug } = Route.useParams()
  return <IssuesTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/issues/')({
  component: IssuesIndexRoute,
})
