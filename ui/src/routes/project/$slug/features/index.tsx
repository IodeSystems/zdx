import { createFileRoute } from '@tanstack/react-router'
import { FeaturesTab } from '../../../../components/FeaturesTab'
import { z } from 'zod'

const searchSchema = z.object({
  category: z.string().optional(),
})

function FeaturesIndexRoute() {
  const { slug } = Route.useParams()
  const { category } = Route.useSearch()
  return <FeaturesTab slug={slug} categoryFilter={category} />
}

export const Route = createFileRoute('/project/$slug/features/')({
  component: FeaturesIndexRoute,
  validateSearch: searchSchema,
})
