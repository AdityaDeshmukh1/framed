/**
 * Profile page — the user's own Cinematic Soul card.
 * Fetches the real soul portrait from GET /soul/:user_id; falls back to mock if unavailable.
 * User stats (age, location, watchlist counts) remain mock until a /profile endpoint is added.
 */

import { PhoneShell } from '../../components/layout/PhoneShell'
import { AppBar } from '../../components/layout/AppBar'
import { BottomNav } from '../../components/layout/BottomNav'
import { SoulPortrait } from '../../components/features/SoulPortrait'
import { PosterGrid } from '../../components/features/PosterGrid'
import { RuleDouble, RuleThin, DiamondDivider } from '../../components/ui/Dividers'
import { auth, currentUser } from '@clerk/nextjs/server'
import { UserButton } from '@clerk/nextjs'
import { POSTER_COLOR_SEQUENCE } from '../../lib/constants'
import { API_BASE_URL } from '../../lib/constants'

/** A single stat cell in the 2×4 stats grid */
function StatCell({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex flex-col gap-0.5 py-3">
      <span
        className="font-mono text-lg font-semibold leading-none"
        style={{ color: 'var(--coral)' }}
      >
        {value}
      </span>
      <span
        className="font-sans text-[11px] uppercase tracking-wider opacity-55"
        style={{ color: 'var(--ink)' }}
      >
        {label}
      </span>
    </div>
  )
}


export default async function ProfilePage() {
  const clerkUser = await currentUser()
  const displayName = clerkUser?.username || clerkUser?.firstName || 'You'

  let portraitText = ''
  try {
    const { getToken } = await auth()
    const token = await getToken()
    const res = await fetch(`${API_BASE_URL}/soul`, {
      headers: { 'Authorization': `Bearer ${token}` },
      cache: 'no-store',
    })
    if (res.ok) {
      const data = await res.json()
      if (data.portrait) portraitText = data.portrait
    }
  } catch {
    // backend unavailable — portrait stays empty
  }

  // Build opening titles list with colours cycling through the palette — mock until prompts endpoint exists
  const userPrompts: Array<{ film_title: string; prompt: string; answer: string }> = []
  const openingTitles = userPrompts.map((p, i) => ({
    title: p.film_title,
    year: undefined as number | undefined,
    color: POSTER_COLOR_SEQUENCE[i % POSTER_COLOR_SEQUENCE.length],
  })) ?? []

  // Pad to 4 entries for the grid
  while (openingTitles.length < 4) {
    const idx = openingTitles.length
    openingTitles.push({
      title: '—',
      year: undefined,
      color: POSTER_COLOR_SEQUENCE[idx % POSTER_COLOR_SEQUENCE.length],
    })
  }

  return (
    <PhoneShell hasBottomNav>
      {/* App bar: back on the left, edit on the right */}
      <AppBar
        showLogo
        rightAction={
          <UserButton />
        }
      />

      <div className="flex flex-col px-5 pt-5 pb-6 gap-6">

        {/* ── Chapter card ─── */}
        <div className="flex flex-col gap-1">
          <h1
            className="font-serif italic text-[38px] leading-none tracking-tight"
            style={{ color: 'var(--ink-deep)' }}
          >
            {displayName}
          </h1>
          <p className="font-mono text-[11px] opacity-40 mt-0.5" style={{ color: 'var(--ink)' }}>
            @{clerkUser?.username ?? ''}
          </p>
        </div>

        <RuleDouble />

        {/* ── Cinematic Soul ─── */}
        {portraitText ? (
          <SoulPortrait portrait={portraitText} showDividers={false} />
        ) : (
          <p className="font-serif italic text-base text-center" style={{ color: 'var(--ink)', opacity: 0.45 }}>
            No portrait yet — complete onboarding to generate yours.
          </p>
        )}

        <RuleDouble />

        {/* ── Opening Titles ─── */}
        <div className="flex flex-col gap-3">
          <p className="chapter-kicker">Opening Titles</p>
          <PosterGrid
            films={openingTitles.slice(0, 4).map((f, i) => ({
              id: String(i),
              title: f.title,
              color: f.color,
            }))}
          />
        </div>

        <RuleThin />

        {/* ── Stats ─── */}
        <div className="flex flex-col gap-2">
          <p className="chapter-kicker">The numbers</p>
          <div
            className="grid grid-cols-2 divide-x divide-y rounded-2xl overflow-hidden"
            style={{ borderColor: 'rgba(74,27,12,0.1)' }}
          >
            <StatCell label="Films watched" value="—" />
            <StatCell label="Obscurity score" value="—" />
            <StatCell label="Avg rating" value="—" />
            <StatCell label="Favourite era" value="—" />
          </div>
        </div>

        <RuleThin />

        {/* ── Film-anchored prompts ─── */}
        {userPrompts.length > 0 && (
          <div className="flex flex-col gap-5">
            <p className="chapter-kicker">In their own words</p>
            {userPrompts.map((prompt) => (
              <div key={prompt.film_title} className="flex flex-col gap-2">
                <p className="font-sans text-xs font-semibold uppercase tracking-wider opacity-50" style={{ color: 'var(--ink)' }}>
                  {prompt.prompt}
                </p>
                <p className="font-serif italic text-base leading-relaxed" style={{ color: 'var(--ink-deep)' }}>
                  &ldquo;{prompt.answer}&rdquo;
                </p>
                <p className="chapter-kicker" style={{ opacity: 0.5 }}>— {prompt.film_title}</p>
              </div>
            ))}
          </div>
        )}

        <DiamondDivider />

        {/* ── Signature footer ─── */}
        <p
          className="text-center font-serif italic text-sm"
          style={{ color: 'var(--ink)', opacity: 0.35 }}
        >
          — end of reel —
        </p>
      </div>

      <BottomNav activeTab="portrait" />
    </PhoneShell>
  )
}
