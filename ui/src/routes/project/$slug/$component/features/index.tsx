import { createFileRoute } from '@tanstack/react-router'
import { FeaturesTab } from '../../../../../components/FeaturesTab'
import { z } from 'zod'

const searchSchema = z.object({
  category: z.string().optional(),
})

function FeaturesIndexRoute() {
  const { slug, component } = Route.useParams()
  const { category } = Route.useSearch()
  return <FeaturesTab slug={slug} componentSlug={component} categoryFilter={category} />
}

export const Route = createFileRoute('/project/$slug/$component/features/')({
  component: FeaturesIndexRoute,
  validateSearch: searchSchema,
})
