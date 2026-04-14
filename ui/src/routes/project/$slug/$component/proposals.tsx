import { createFileRoute } from '@tanstack/react-router'
import { ProposalsTab } from '../../../../components/ProposalsTab'

function ProposalsRoute() {
  const { slug } = Route.useParams()
  return <ProposalsTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/$component/proposals')({
  component: ProposalsRoute,
})
