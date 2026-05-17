'use client'

/**
 * BottomNav — 4-tab navigation bar: portrait, discover, matches, chat.
 * Highlights the active tab based on the current route.
 */

import Link from 'next/link'
import { usePathname } from 'next/navigation'

type TabKey = 'portrait' | 'discover' | 'matches' | 'chat' | 'siblings'

/** Props for the BottomNav component */
interface Props {
  /** Force-active tab (overrides pathname matching) */
  activeTab?: TabKey
}

interface Tab {
  key: TabKey
  href: string
  label: string
  icon: React.ReactNode
}

function PortraitIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <rect x="4" y="2" width="14" height="18" rx="2" />
      <circle cx="11" cy="9" r="3" />
      <path d="M6 18c0-2.8 2.2-5 5-5s5 2.2 5 5" />
    </svg>
  )
}

function DiscoverIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="11" cy="11" r="8" />
      <path d="M14.5 7.5L12 12L7.5 14.5L10 10L14.5 7.5Z" />
    </svg>
  )
}

function MatchesIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 19C11 19 3 14 3 8.5C3 6 5 4 7.5 4C9 4 10.3 4.7 11 5.8C11.7 4.7 13 4 14.5 4C17 4 19 6 19 8.5C19 14 11 19 11 19Z" />
    </svg>
  )
}

function ChatIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 4h14a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H8l-4 4V6a2 2 0 0 1 2-2Z" />
    </svg>
  )
}

const TABS: Tab[] = [
  { key: 'portrait', href: '/profile', label: 'Portrait', icon: <PortraitIcon /> },
  { key: 'discover', href: '/discover', label: 'Discover', icon: <DiscoverIcon /> },
  { key: 'matches', href: '/siblings', label: 'Siblings', icon: <MatchesIcon /> },
  { key: 'chat', href: '/chat/match-001', label: 'Chat', icon: <ChatIcon /> },
]

export function BottomNav({ activeTab }: Props) {
  const pathname = usePathname()

  function isActive(tab: Tab): boolean {
    if (activeTab) return activeTab === tab.key
    return pathname.startsWith(tab.href)
  }

  return (
    <nav
      className="fixed bottom-0 left-1/2 -translate-x-1/2 w-full max-w-[390px] flex items-center justify-around px-2 pb-safe-bottom pt-2 z-20"
      style={{
        background: 'var(--cream)',
        borderTop: '1px solid rgba(74,27,12,0.15)',
      }}
    >
      {TABS.map((tab) => {
        const active = isActive(tab)
        return (
          <Link
            key={tab.key}
            href={tab.href}
            className="flex flex-col items-center gap-0.5 py-2 px-4 rounded-xl transition-opacity"
            style={{
              color: active ? 'var(--coral)' : 'var(--ink)',
              opacity: active ? 1 : 0.45,
            }}
            aria-label={tab.label}
            aria-current={active ? 'page' : undefined}
          >
            {tab.icon}
            <span className="font-sans text-[10px] font-medium tracking-wide">
              {tab.label}
            </span>
          </Link>
        )
      })}
    </nav>
  )
}
