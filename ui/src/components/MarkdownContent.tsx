import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link } from '@tanstack/react-router'
import { Box } from '@mui/material'

const ENTITY_RE = /\b(IS-\d+|TK-\d+)\b/g

function linkifyEntities(text: string, slug: string): string {
  return text.replace(ENTITY_RE, (match) => {
    const prefix = match.startsWith('IS-') ? 'issues' : 'tasks'
    return `[${match}](/project/${encodeURIComponent(slug)}/${prefix}/${match})`
  })
}

export function MarkdownContent({
  children,
  slug,
  variant = 'body2',
  color,
}: {
  children: string
  slug: string
  variant?: 'body1' | 'body2'
  color?: string
}) {
  const processed = linkifyEntities(children, slug)

  return (
    <Box sx={{
      fontSize: variant === 'body1' ? '1rem' : '0.875rem',
      color,
      '& p': { m: 0, mb: 0.5 },
      '& p:last-child': { mb: 0 },
      '& ul, & ol': { m: 0, mb: 0.5, pl: 2.5 },
      '& li': { mb: 0.25 },
      '& code': { bgcolor: 'action.hover', px: 0.5, borderRadius: 0.5, fontSize: '0.85em' },
      '& pre': { m: 0, mb: 0.5, '& code': { bgcolor: 'transparent', p: 0 } },
      '& pre > code': { display: 'block', p: 1, bgcolor: 'action.hover', borderRadius: 1, overflow: 'auto' },
      '& blockquote': { m: 0, mb: 0.5, pl: 1.5, borderLeft: 3, borderColor: 'divider', color: 'text.secondary' },
      '& h1, & h2, & h3, & h4, & h5, & h6': { mt: 1, mb: 0.5, fontSize: '1em', fontWeight: 600 },
      '& table': { borderCollapse: 'collapse', mb: 0.5, fontSize: '0.85em' },
      '& td, & th': { border: 1, borderColor: 'divider', px: 1, py: 0.25 },
      '& a': { color: 'primary.main', textDecoration: 'none', '&:hover': { textDecoration: 'underline' } },
    }}>
      <Markdown
        remarkPlugins={[remarkGfm]}
        components={{
          a({ href, children }) {
            if (href?.startsWith('/project/')) {
              return <Link to={href} style={{ color: 'inherit' }}>{children}</Link>
            }
            return <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>
          },
        }}
      >
        {processed}
      </Markdown>
    </Box>
  )
}
