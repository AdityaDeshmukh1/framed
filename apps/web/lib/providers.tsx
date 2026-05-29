'use client'

/**
 * ReactQueryProvider — wraps the app in TanStack Query context.
 * Must be a client component because QueryClient uses browser state.
 */

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'

interface Props {
  /** Child components that will have access to the query client */
  children: React.ReactNode
}

export function ReactQueryProvider({ children }: Props) {
  // Create a stable QueryClient instance per component mount
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60 * 1000, // 1 minute
            retry: 1,
          },
        },
      })
  )

  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}
