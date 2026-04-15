import { useEffect, useRef, useCallback, useLayoutEffect } from 'react'
import { getToken } from '../api/client'

type MessageHandler = (msg: { channel: string; type: string; payload: unknown }) => void

const RECONNECT_DELAY = 2000

type ChannelSocket = {
  socket: WebSocket | null
  handlers: Set<MessageHandler>
  reconnectTimer: ReturnType<typeof setTimeout> | null
}

const channels = new Map<string, ChannelSocket>()

function getWsUrl(token: string): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/api/ws/subscribe?token=${encodeURIComponent(token)}`
}

async function signChannel(channel: string): Promise<string> {
  const apiToken = getToken()
  if (!apiToken) throw new Error('not authenticated')
  const res = await fetch('/api/ws/sign', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Api-Key': apiToken },
    body: JSON.stringify({ channel }),
  })
  if (!res.ok) throw new Error(`sign failed: ${res.status}`)
  const data = await res.json()
  return data.token
}

function connectChannel(channel: string) {
  const entry = channels.get(channel)
  if (!entry || entry.handlers.size === 0) return
  if (entry.socket && (entry.socket.readyState === WebSocket.OPEN || entry.socket.readyState === WebSocket.CONNECTING)) return

  signChannel(channel).then(token => {
    const currentEntry = channels.get(channel)
    if (!currentEntry || currentEntry.handlers.size === 0) return

    const ws = new WebSocket(getWsUrl(token))
    currentEntry.socket = ws

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        const e = channels.get(channel)
        if (e) e.handlers.forEach(h => h(msg))
      } catch { /* ignore malformed */ }
    }

    ws.onclose = () => {
      const e = channels.get(channel)
      if (e) {
        e.socket = null
        if (e.handlers.size > 0 && !e.reconnectTimer) {
          e.reconnectTimer = setTimeout(() => {
            const e2 = channels.get(channel)
            if (e2) {
              e2.reconnectTimer = null
              if (e2.handlers.size > 0) connectChannel(channel)
            }
          }, RECONNECT_DELAY)
        }
      }
    }
  }).catch(() => {})
}

export function useChannel(channel: string | null, onMessage: MessageHandler) {
  const handlerRef = useRef(onMessage)
  useLayoutEffect(() => {
    handlerRef.current = onMessage
  })

  const stableHandler = useCallback((msg: { channel: string; type: string; payload: unknown }) => {
    handlerRef.current(msg)
  }, [])

  useEffect(() => {
    if (!channel) return

    if (!channels.has(channel)) {
      channels.set(channel, { socket: null, handlers: new Set(), reconnectTimer: null })
    }
    const entry = channels.get(channel)!
    entry.handlers.add(stableHandler)
    connectChannel(channel)

    return () => {
      const e = channels.get(channel)
      if (e) {
        e.handlers.delete(stableHandler)
        if (e.handlers.size === 0) {
          if (e.reconnectTimer) {
            clearTimeout(e.reconnectTimer)
            e.reconnectTimer = null
          }
          e.socket?.close()
          e.socket = null
          channels.delete(channel)
        }
      }
    }
  }, [channel, stableHandler])
}
