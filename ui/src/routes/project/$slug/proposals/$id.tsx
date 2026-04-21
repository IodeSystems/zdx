import { createFileRoute } from '@tanstack/react-router'
import { ProposalDetail } from '../../../../components/ProposalDetail'

function ProposalDetailRoute() {
  const { slug, id } = Route.useParams()
  const n = Number(id)
  return <ProposalDetail slug={slug} proposalId={Number.isFinite(n) ? n : 0} />
}

export const Route = createFileRoute('/project/$slug/proposals/$id')({
  component: ProposalDetailRoute,
})
