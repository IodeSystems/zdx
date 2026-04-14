import { createFileRoute } from '@tanstack/react-router'
import { ThemesTab } from '../../../../components/ThemesTab'

function ThemesIndexRoute() {
  const { slug } = Route.useParams()
  return <ThemesTab slug={slug} />
}

export const Route = createFileRoute('/project/$slug/themes/')({
  component: ThemesIndexRoute,
})
