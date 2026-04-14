import { useEffect, useRef, useCallback } from 'react'
import { getToken } from '../api/client'

type MessageHandler = (msg: { channel: string; type: string; payload: unknown }) => void

const RECONNECT_DELAY = 2000

let sharedSocket: WebSocket | null = null
let subscriberCount = 0
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
const handlers = new Map<string, Set<MessageHandler>>()

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

function ensureConnection(token: string) {
  if (sharedSocket && (sharedSocket.readyState === WebSocket.OPEN || sharedSocket.readyState === WebSocket.CONNECTING)) {
    return
  }

  sharedSocket = new WebSocket(getWsUrl(token))

  sharedSocket.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      const channelHandlers = handlers.get(msg.channel)
      if (channelHandlers) {
        channelHandlers.forEach((h) => h(msg))
      }
    } catch {
      // ignore malformed messages
    }
  }

  sharedSocket.onclose = () => {
    sharedSocket = null
    if (subscriberCount > 0 && !reconnectTimer) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        if (subscriberCount > 0) {
          signChannel([...handlers.keys()][0] || '').then(ensureConnection).catch(() => {})
        }
      }, RECONNECT_DELAY)
    }
  }
}

export function useChannel(channel: string | null, onMessage: MessageHandler) {
  const handlerRef = useRef(onMessage)
  handlerRef.current = onMessage

  const stableHandler = useCallback((msg: { channel: string; type: string; payload: unknown }) => {
    handlerRef.current(msg)
  }, [])

  useEffect(() => {
    if (!channel) return

    subscriberCount++
    if (!handlers.has(channel)) {
      handlers.set(channel, new Set())
    }
    handlers.get(channel)!.add(stableHandler)

    signChannel(channel)
      .then(ensureConnection)
      .catch(() => {})

    return () => {
      const set = handlers.get(channel)
      if (set) {
        set.delete(stableHandler)
        if (set.size === 0) handlers.delete(channel)
      }
      subscriberCount--
      if (subscriberCount === 0) {
        if (reconnectTimer) {
          clearTimeout(reconnectTimer)
          reconnectTimer = null
        }
        sharedSocket?.close()
        sharedSocket = null
      }
    }
  }, [channel, stableHandler])
}
