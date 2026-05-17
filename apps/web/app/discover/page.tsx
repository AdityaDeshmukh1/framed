'use client'

/**
 * Discover page — the match feed showing potential romantic connections.
 * Fetches matches via useMatches hook; falls back to mock data for development.
 */

import { motion } from 'framer-motion'
import { PhoneShell } from '../../components/layout/PhoneShell'
import { AppBar } from '../../components/layout/AppBar'
import { BottomNav } from '../../components/layout/BottomNav'
import { MatchCard } from '../../components/features/MatchCard'
import { Badge } from '../../components/ui/Badge'
import { RuleDouble } from '../../components/ui/Dividers'
import { MOCK_MATCHES } from '../../lib/constants'

function SettingsIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--ink)', opacity: 0.6 }}>
      <circle cx="10" cy="10" r="2.5" />
      <path d="M10 2v1.5M10 16.5V18M2 10h1.5M16.5 10H18M4.1 4.1l1.1 1.1M14.8 14.8l1.1 1.1M4.1 15.9l1.1-1.1M14.8 5.2l1.1-1.1" />
    </svg>
  )
}

export default function DiscoverPage() {
  // Use mock data directly since the backend may not be running
  const matches = MOCK_MATCHES
  const isLoading = false

  return (
    <PhoneShell hasBottomNav>
      <AppBar
        showLogo
        rightAction={
          <button
            type="button"
            className="p-2 rounded-full hover:bg-black/5 transition-colors"
            aria-label="Settings"
          >
            <SettingsIcon />
          </button>
        }
      />

      <div className="flex flex-col flex-1 px-5 pt-4 pb-4 gap-5">
        {/* Section header */}
        <div className="flex flex-col gap-3">
          <RuleDouble />
          <div className="flex items-center justify-between">
            <p className="chapter-kicker">now showing · your matches</p>
            <Badge variant="count">{matches.length}</Badge>
          </div>
          <RuleDouble />
        </div>

        {/* Match feed */}
        {isLoading ? (
          <div className="flex flex-col gap-4">
            {[1, 2, 3].map((i) => (
              <div
                key={i}
                className="rounded-2xl h-48 animate-pulse"
                style={{ background: 'var(--cream-2)' }}
              />
            ))}
          </div>
        ) : (
          <motion.div
            className="flex flex-col gap-4"
            initial="hidden"
            animate="visible"
            variants={{
              hidden: {},
              visible: { transition: { staggerChildren: 0.1 } },
            }}
          >
            {matches.map((match) => (
              <motion.div
                key={match.id}
                variants={{
                  hidden: { opacity: 0, y: 12 },
                  visible: { opacity: 1, y: 0 },
                }}
                transition={{ duration: 0.35, ease: 'easeOut' }}
              >
                <MatchCard
                  id={match.id}
                  score={match.score}
                  why_text={match.why_text}
                  shared_films={match.shared_films}
                  other_user={match.other_user}
                />
              </motion.div>
            ))}
          </motion.div>
        )}

        {/* Footer note */}
        <p
          className="text-center font-sans text-xs py-4"
          style={{ color: 'var(--ink)', opacity: 0.3 }}
        >
          — end of reel —
        </p>
      </div>

      <BottomNav activeTab="discover" />
    </PhoneShell>
  )
}
