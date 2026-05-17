'use client'

/**
 * StatusBar — simulates a mobile status bar with time, title, and signal.
 * Uses client component because time is dynamic.
 */

import { useEffect, useState } from 'react'

/** Props for the StatusBar component */
interface Props {
  /** Optional theme override — defaults to 'dark' (white text) */
  theme?: 'dark' | 'light'
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: false,
  })
}

export function StatusBar({ theme = 'dark' }: Props) {
  const [time, setTime] = useState(formatTime(new Date()))

  useEffect(() => {
    const interval = setInterval(() => {
      setTime(formatTime(new Date()))
    }, 10_000)
    return () => clearInterval(interval)
  }, [])

  const textColor = theme === 'dark' ? 'text-[var(--cream)]' : 'text-[var(--ink-deep)]'

  return (
    <div className={`flex items-center justify-between px-5 pt-3 pb-1 ${textColor}`}>
      {/* Time */}
      <span className="font-mono text-xs font-semibold tabular-nums tracking-tight">
        {time}
      </span>

      {/* Center — "Framed" wordmark */}
      <span className="font-serif italic text-sm tracking-widest opacity-70">
        Framed
      </span>

      {/* Signal indicators */}
      <div className="flex items-center gap-1 opacity-80">
        {/* WiFi bars */}
        <svg width="15" height="11" viewBox="0 0 15 11" fill="currentColor" aria-hidden="true">
          <path d="M7.5 2.5C9.5 2.5 11.3 3.3 12.6 4.6L14 3.2C12.3 1.5 10 0.5 7.5 0.5C5 0.5 2.7 1.5 1 3.2L2.4 4.6C3.7 3.3 5.5 2.5 7.5 2.5Z" opacity="0.5"/>
          <path d="M7.5 5.5C8.8 5.5 9.9 6 10.7 6.8L12.1 5.4C10.9 4.2 9.3 3.5 7.5 3.5C5.7 3.5 4.1 4.2 2.9 5.4L4.3 6.8C5.1 6 6.2 5.5 7.5 5.5Z" opacity="0.7"/>
          <circle cx="7.5" cy="9.5" r="1.5"/>
        </svg>
        {/* Battery */}
        <svg width="20" height="11" viewBox="0 0 20 11" fill="currentColor" aria-hidden="true">
          <rect x="0.5" y="0.5" width="16" height="10" rx="2" stroke="currentColor" strokeWidth="1" fill="none" opacity="0.6"/>
          <rect x="2" y="2" width="11" height="7" rx="1" opacity="0.8"/>
          <path d="M17.5 3.5V7.5C18.3 7.2 19 6.4 19 5.5C19 4.6 18.3 3.8 17.5 3.5Z" opacity="0.6"/>
        </svg>
      </div>
    </div>
  )
}
