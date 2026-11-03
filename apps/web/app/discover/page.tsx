'use client'

/**
 * Discover page — the match feed showing potential connections.
 * Fetches real matches from GET /api/matches/[userId].
 */

import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { PhoneShell } from '../../components/layout/PhoneShell'
import { AppBar } from '../../components/layout/AppBar'
import { BottomNav } from '../../components/layout/BottomNav'
import { MatchCard } from '../../components/features/MatchCard'
import { Badge } from '../../components/ui/Badge'
import { RuleDouble } from '../../components/ui/Dividers'
import { fetchMatchList } from '../../lib/api'
import type { MatchListItem } from '../../lib/types'

function SettingsIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--ink)', opacity: 0.6 }}>
      <circle cx="10" cy="10" r="2.5" />
      <path d="M10 2v1.5M10 16.5V18M2 10h1.5M16.5 10H18M4.1 4.1l1.1 1.1M14.8 14.8l1.1 1.1M4.1 15.9l1.1-1.1M14.8 5.2l1.1-1.1" />
    </svg>
  )
}

/** Transform backend MatchListItem into the shape MatchCard expects. */
function toMatchCardProps(item: MatchListItem) {
  const { location_city, location_country } = item.other_user
  const location = [location_city, location_country].filter(Boolean).join(', ')
  return {
    id:       item.id,
    score:    item.score,
    why_text: item.why_text || undefined,
    shared_films: [], // not available in list view — shown in detail
    other_user: {
      display_name:      item.other_user.display_name,
      letterboxd_handle: item.other_user.letterboxd_handle,
      age:               item.other_user.age || undefined,
      location:          location || undefined,
      films_watched:     undefined,
    },
  }
}

export default function DiscoverPage() {
  const [matches, setMatches] = useState<MatchListItem[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    console.log('[framed:discover] fetching matches')
    fetchMatchList()
      .then((data) => {
        console.log(`[framed:discover] loaded ${data.length} matches`)
        setMatches(data)
      })
      .catch((err) => {
        console.error('[framed:discover] fetch failed:', err)
        setError('Could not load matches — is the backend running?')
      })
      .finally(() => setIsLoading(false))
  }, [])

  return (
    <PhoneShell hasBottomNav>
      <AppBar
        showLogo
        rightAction={
          <button type="button" className="p-2 rounded-full hover:bg-black/5 transition-colors" aria-label="Settings">
            <SettingsIcon />
          </button>
        }
      />

      <div className="flex flex-col flex-1 px-5 pt-4 pb-4 gap-5">
        <div className="flex flex-col gap-3">
          <RuleDouble />
          <div className="flex items-center justify-between">
            <p className="chapter-kicker">now showing · your matches</p>
            {!isLoading && <Badge variant="count">{matches.length}</Badge>}
          </div>
          <RuleDouble />
        </div>

        {isLoading && (
          <div className="flex flex-col gap-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="rounded-2xl h-48 animate-pulse" style={{ background: 'var(--cream-2)' }} />
            ))}
          </div>
        )}

        {!isLoading && error && (
          <div className="flex flex-col items-center gap-3 py-12">
            <p className="font-serif italic text-base text-center" style={{ color: 'var(--ink)', opacity: 0.5 }}>
              {error}
            </p>
            <p className="font-mono text-xs text-center" style={{ color: 'var(--ink)', opacity: 0.3 }}>
              POST /match first to compute matches
            </p>
          </div>
        )}

        {!isLoading && !error && matches.length === 0 && (
          <div className="flex flex-col items-center gap-3 py-12">
            <p className="font-serif italic text-base text-center" style={{ color: 'var(--ink)', opacity: 0.5 }}>
              No matches yet.
            </p>
            <p className="font-sans text-xs text-center" style={{ color: 'var(--ink)', opacity: 0.35 }}>
              Complete your portrait first — matches are computed automatically.
            </p>
          </div>
        )}

        {!isLoading && !error && matches.length > 0 && (
          <motion.div
            className="flex flex-col gap-4"
            initial="hidden"
            animate="visible"
            variants={{ hidden: {}, visible: { transition: { staggerChildren: 0.1 } } }}
          >
            {matches.map((match) => (
              <motion.div
                key={match.id}
                variants={{ hidden: { opacity: 0, y: 12 }, visible: { opacity: 1, y: 0 } }}
                transition={{ duration: 0.35, ease: 'easeOut' }}
              >
                <MatchCard {...toMatchCardProps(match)} />
              </motion.div>
            ))}
          </motion.div>
        )}

        {!isLoading && !error && (
          <p className="text-center font-sans text-xs py-4" style={{ color: 'var(--ink)', opacity: 0.3 }}>
            — end of reel —
          </p>
        )}
      </div>

      <BottomNav activeTab="discover" />
    </PhoneShell>
  )
}
