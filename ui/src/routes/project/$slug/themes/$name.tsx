import { createFileRoute } from '@tanstack/react-router'
import { ThemeDetail } from '../../../../components/ThemeDetail'

function ThemeDetailRoute() {
  const { slug, name } = Route.useParams()
  return <ThemeDetail slug={slug} name={decodeURIComponent(name)} />
}

export const Route = createFileRoute('/project/$slug/themes/$name')({
  component: ThemeDetailRoute,
})
