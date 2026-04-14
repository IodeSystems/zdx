import { createContext, useContext, useState, useCallback, type ReactNode } from 'react'

interface ComponentContextValue {
  component: string
  setComponent: (component: string) => void
}

const ComponentContext = createContext<ComponentContextValue>({
  component: '',
  setComponent: () => {},
})

function storageKey(slug: string) {
  return `zdx-component-${slug}`
}

export function ComponentProvider({ slug, children }: { slug: string; children: ReactNode }) {
  const [component, setComponentState] = useState(() => {
    try {
      return sessionStorage.getItem(storageKey(slug)) || ''
    } catch { /* sessionStorage unavailable */
      return ''
    }
  })

  const setComponent = useCallback(
    (value: string) => {
      setComponentState(value)
      try {
        if (value) {
          sessionStorage.setItem(storageKey(slug), value)
        } else {
          sessionStorage.removeItem(storageKey(slug))
        }
      } catch { /* sessionStorage unavailable */ }
    },
    [slug],
  )

  return <ComponentContext.Provider value={{ component, setComponent }}>{children}</ComponentContext.Provider>
}

export function useComponentFilter() {
  return useContext(ComponentContext)
}
