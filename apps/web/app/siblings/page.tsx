/**
 * Soul Siblings page — taste-compatible connections without romantic framing.
 * Server component using mock data; real data would come from the matches endpoint
 * filtered by score threshold and community intent.
 */

import { PhoneShell } from '../../components/layout/PhoneShell'
import { AppBar } from '../../components/layout/AppBar'
import { BottomNav } from '../../components/layout/BottomNav'
import { SiblingCard } from '../../components/features/SiblingCard'
import { RuleDouble, DiamondDivider } from '../../components/ui/Dividers'
import { Badge } from '../../components/ui/Badge'

function SettingsIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--ink)', opacity: 0.6 }}>
      <circle cx="10" cy="10" r="2.5" />
      <path d="M10 2v1.5M10 16.5V18M2 10h1.5M16.5 10H18M4.1 4.1l1.1 1.1M14.8 14.8l1.1 1.1M4.1 15.9l1.1-1.1M14.8 5.2l1.1-1.1" />
    </svg>
  )
}

export default function SiblingsPage() {
  const siblings: Array<{ id: string; display_name: string; age: number; location: string; letterboxd_handle: string; score: number; shared_trait: string; films_watched: number }> = []

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

      <div className="flex flex-col px-5 pt-5 pb-6 gap-5">

        {/* ── Section header ─── */}
        <div className="flex flex-col gap-3">
          <RuleDouble />
          <div className="flex items-baseline justify-between">
            <div className="flex flex-col gap-0.5">
              <h1
                className="font-serif italic text-3xl leading-none"
                style={{ color: 'var(--ink-deep)' }}
              >
                Soul Siblings
              </h1>
              <p
                className="font-sans text-xs opacity-50 italic mt-1"
                style={{ color: 'var(--ink)' }}
              >
                no romance attached
              </p>
            </div>
            <Badge variant="count">{siblings.length}</Badge>
          </div>
          <RuleDouble />
        </div>

        {/* ── Description ─── */}
        <p
          className="font-sans text-sm leading-relaxed opacity-60"
          style={{ color: 'var(--ink)' }}
        >
          People who share your cinematic vocabulary. Not a match — a frequency.
        </p>

        {/* ── Siblings list ─── */}
        <div className="flex flex-col gap-4">
          {siblings.map((sibling) => (
            <SiblingCard
              key={sibling.id}
              display_name={sibling.display_name}
              age={sibling.age}
              location={sibling.location}
              letterboxd_handle={sibling.letterboxd_handle}
              score={sibling.score}
              shared_trait={sibling.shared_trait}
              films_watched={sibling.films_watched}
            />
          ))}
        </div>

        <DiamondDivider />

        <p
          className="text-center font-sans text-xs py-2"
          style={{ color: 'var(--ink)', opacity: 0.3 }}
        >
          — end of reel —
        </p>
      </div>

      <BottomNav />
    </PhoneShell>
  )
}
