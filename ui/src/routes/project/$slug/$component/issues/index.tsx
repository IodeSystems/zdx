import { createFileRoute } from '@tanstack/react-router'
import { IssuesTab } from '../../../../../components/IssuesTab'

function IssuesIndexRoute() {
  const { slug, component } = Route.useParams()
  return <IssuesTab slug={slug} componentSlug={component} />
}

export const Route = createFileRoute('/project/$slug/$component/issues/')({
  component: IssuesIndexRoute,
})
