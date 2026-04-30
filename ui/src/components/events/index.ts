import type React from 'react'
import type { EventItem } from '../../api'
export { CommentEvent } from './CommentEvent'
export { RevisionEvent } from './RevisionEvent'
export { StatusChangeEvent } from './StatusChangeEvent'

import { CommentEvent } from './CommentEvent'
import { RevisionEvent } from './RevisionEvent'
import { StatusChangeEvent } from './StatusChangeEvent'

export const EVENT_RENDERERS: Record<string, React.ComponentType<{ event: EventItem; slug: string }>> = {
  comment: CommentEvent,
  revision: RevisionEvent,
  status_change: StatusChangeEvent,
}
