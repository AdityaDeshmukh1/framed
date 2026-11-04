'use client'

/**
 * Onboarding — the rating quiz is the whole thing (≈60–75s):
 *
 *   rate ~10 films → "your taste so far" → three tap-only basics → /portrait
 *
 * Cards come from GET /quiz/pool and are picked on-device by
 * lib/quiz/engine.ts. Answers go to POST /onboard {source: "quiz"} as soon as
 * the last card is rated, so the taste vector is being built while the user
 * looks at the reveal and fills in the basics. /portrait then polls the job,
 * writes the portrait and computes matches, as it does for Letterboxd imports.
 *
 * Gestures: swipe right = loved, left = not for me, up = it's fine,
 * double-tap = all-time favourite. Keys: → ← ↑, F favourite, ↓ haven't seen,
 * Z undo, D show the engine's reasoning.
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { AnimatePresence, motion } from 'framer-motion'
import { Heart, Meh, RotateCcw, ThumbsDown, EyeOff, Sigma, Star } from 'lucide-react'
import { PhoneShell } from '../../components/layout/PhoneShell'
import { Button } from '../../components/ui/Button'
import { RuleDouble } from '../../components/ui/Dividers'
import { QuizCard } from '../../components/features/QuizCard'
import { saveBasics, startQuizOnboarding } from '../../lib/api'
import {
  type Answer,
  type Model,
  type Pool,
  type QuizState,
  type SeedFilm,
  MIN_RATED,
  TARGET_RATED,
  buildPool,
  explain,
  fitModel,
  genreAffinity,
  isDone,
  POSTER_SIZES,
  posterSrcSet,
  posterUrl,
  ratedCount,
  toSubmission,
} from '../../lib/quiz/engine'

type Step = 'rate' | 'reveal' | 'basics'
const EMPTY: QuizState = { favourites: [], responses: [] }

export default function OnboardingPage() {
  const router = useRouter()
  const [pool, setPool] = useState<Pool | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [step, setStep] = useState<Step>('rate')
  const [state, setState] = useState<QuizState>(EMPTY)
  const [target, setTarget] = useState(TARGET_RATED)
  const [jobId, setJobId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/quiz/pool')
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((seed: SeedFilm[]) => {
        if (!seed.length) throw new Error('the quiz pool is empty (run cmd/catalogue)')
        setPool(buildPool(seed))
      })
      .catch((e) => setLoadError(String(e)))
  }, [])

  /** Send answers now; the pipeline runs while the user reads the reveal. */
  const submit = useCallback(
    async (s: QuizState) => {
      if (!pool) return
      setSubmitError(null)
      try {
        const { job_id } = await startQuizOnboarding(toSubmission(pool, s))
        setJobId(job_id)
      } catch (e) {
        setSubmitError(e instanceof Error ? e.message : String(e))
      }
    },
    [pool],
  )

  const finishRating = useCallback(
    (s: QuizState) => {
      setStep('reveal')
      void submit(s)
    },
    [submit],
  )

  return (
    <PhoneShell>
      <div className="flex flex-col flex-1 px-6 pt-6 pb-6">
        {loadError ? (
          <div className="font-sans text-sm flex flex-col gap-2 m-auto text-center">
            <p className="font-semibold">Couldn&apos;t load films.</p>
            <p style={{ opacity: 0.6 }}>{loadError}</p>
          </div>
        ) : !pool ? (
          <p className="font-sans text-sm m-auto" style={{ opacity: 0.5 }}>
            Loading films…
          </p>
        ) : step === 'rate' ? (
          <Rate pool={pool} state={state} setState={setState} target={target} onDone={finishRating} />
        ) : step === 'reveal' ? (
          <Reveal
            pool={pool}
            state={state}
            submitError={submitError}
            onRetry={() => submit(state)}
            onMore={() => {
              setTarget((t) => t + 10)
              setStep('rate')
            }}
            onNext={() => setStep('basics')}
          />
        ) : (
          <Basics
            ready={!!jobId}
            onDone={async (basics) => {
              await saveBasics(basics)
              router.push(`/portrait?jobId=${jobId}`)
            }}
          />
        )}
      </div>
    </PhoneShell>
  )
}

/* ── Shared ─────────────────────────────────────────────────────────────── */

function Header({ kicker, title, sub }: { kicker: string; title: string; sub?: string }) {
  return (
    <div className="flex flex-col gap-3">
      <RuleDouble />
      <p className="chapter-kicker">{kicker}</p>
      <h1 className="font-serif italic text-4xl leading-none" style={{ color: 'var(--ink-deep)' }}>
        {title}
      </h1>
      {sub && (
        <p className="font-sans text-sm" style={{ opacity: 0.6 }}>
          {sub}
        </p>
      )}
      <RuleDouble />
    </div>
  )
}

/* ── Rate ───────────────────────────────────────────────────────────────── */

function Rate({
  pool,
  state,
  setState,
  target,
  onDone,
}: {
  pool: Pool
  state: QuizState
  setState: React.Dispatch<React.SetStateAction<QuizState>>
  target: number
  onDone: (s: QuizState) => void
}) {
  const model = useMemo(() => fitModel(pool, state), [pool, state])
  const current = model.predictions[0]
  const reason = current ? explain(model, current) : ''
  const rated = ratedCount(state)
  const [exitDir, setExitDir] = useState<'left' | 'right' | 'up' | 'down'>('right')
  const [showMaths, setShowMaths] = useState(false)

  const answer = useCallback(
    (a: Answer) => {
      if (!current) return
      setExitDir(a === 'loved' || a === 'favourite' ? 'right' : a === 'nope' ? 'left' : a === 'fine' ? 'up' : 'down')
      const next = {
        ...state,
        responses: [...state.responses, { id: current.film.tmdbId, answer: a, followUp: !!current.followUpOf }],
      }
      setState(next)
      if (isDone(fitModel(pool, next), next, target)) setTimeout(() => onDone(next), 250)
    },
    [current, state, setState, onDone, pool, target],
  )

  const undo = useCallback(() => setState((s) => ({ ...s, responses: s.responses.slice(0, -1) })), [setState])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const map: Record<string, Answer> = { ArrowRight: 'loved', ArrowLeft: 'nope', ArrowUp: 'fine', ArrowDown: 'unseen', f: 'favourite' }
      if (map[e.key]) {
        e.preventDefault()
        answer(map[e.key])
      } else if (e.key === 'z' || e.key === 'Backspace') undo()
      else if (e.key === 'd') setShowMaths((v) => !v)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [answer, undo])

  const exit = {
    right: { x: 400, rotate: 15, opacity: 0 },
    left: { x: -400, rotate: -15, opacity: 0 },
    up: { y: -400, opacity: 0 },
    down: { y: 300, opacity: 0, scale: 0.9 },
  }[exitDir]

  return (
    <div className="flex flex-col gap-4 h-[calc(100svh-3rem)] min-h-0">
      <div className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between font-sans">
          <p className="chapter-kicker">{rated === 0 ? 'Rate a few films' : rated < target ? `${target - rated} to go` : 'Done'}</p>
          <Link href="/onboarding/letterboxd" className="text-xs underline underline-offset-2" style={{ opacity: 0.55 }}>
            Have Letterboxd?
          </Link>
        </div>
        <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--cream-2)' }}>
          <motion.div
            className="h-full rounded-full"
            style={{ background: 'var(--coral)' }}
            animate={{ width: `${Math.round(Math.min(1, rated / target) * 100)}%` }}
            transition={{ duration: 0.3 }}
          />
        </div>
      </div>

      {/* Always poster-shaped (2:3) and as large as the space allows: stretching
          the card to fill the space made object-cover crop the posters. */}
      <div className="relative flex-1 min-h-0 flex items-center justify-center" style={{ containerType: 'size' }}>
        <div className="relative" style={{ width: 'min(100cqw, calc(100cqh * 2 / 3))', aspectRatio: '2 / 3' }}>
        <AnimatePresence>
          {current && (
            <motion.div
              key={current.film.tmdbId}
              className="absolute inset-0"
              initial={{ scale: 0.95, opacity: 0, y: 10 }}
              animate={{ scale: 1, opacity: 1, y: 0 }}
              exit={exit}
              transition={{ duration: 0.22 }}
            >
              <QuizCard
                film={current.film}
                reason={reason}
                onSwipe={(d) => answer(d === 'right' ? 'loved' : d === 'left' ? 'nope' : 'fine')}
                onFavourite={() => answer('favourite')}
              />
            </motion.div>
          )}
        </AnimatePresence>
        </div>
        {/* Preload the likely next posters so cards never wait on the network. */}
        <div className="hidden" aria-hidden>
          {model.predictions.slice(1, 5).map((p) => (
            // eslint-disable-next-line @next/next/no-img-element
            <img key={p.film.tmdbId} src={posterUrl(p.film.posterPath)} srcSet={posterSrcSet(p.film.posterPath)} sizes={POSTER_SIZES} alt="" />
          ))}
        </div>
      </div>

      {showMaths && current && <Maths model={model} />}

      <div className="grid grid-cols-4 gap-2">
        <AnswerButton icon={<ThumbsDown size={20} />} label="Not for me" onClick={() => answer('nope')} bg="var(--ink-deep)" />
        <AnswerButton icon={<Meh size={20} />} label="It's fine" onClick={() => answer('fine')} bg="var(--amber)" fg="var(--ink-deep)" />
        <AnswerButton icon={<Heart size={20} />} label="Loved it" onClick={() => answer('loved')} bg="var(--teal)" />
        <AnswerButton icon={<Star size={20} />} label="All-time" onClick={() => answer('favourite')} bg="var(--coral)" />
      </div>
      <div className="flex items-center justify-between font-sans text-sm">
        <button onClick={undo} disabled={!state.responses.length} className="flex items-center gap-1.5 disabled:opacity-30" style={{ opacity: 0.7 }}>
          <RotateCcw size={14} /> Undo
        </button>
        <button onClick={() => answer('unseen')} className="flex items-center gap-1.5 font-semibold" style={{ color: 'var(--coral)' }}>
          <EyeOff size={14} /> Haven&apos;t seen it
        </button>
        <button onClick={() => onDone(state)} disabled={rated < MIN_RATED} className="disabled:opacity-30" style={{ opacity: 0.7 }}>
          Finish
        </button>
      </div>
      <button onClick={() => setShowMaths((v) => !v)} className="flex items-center justify-center gap-1 font-mono text-[10px]" style={{ color: 'var(--coral)', opacity: 0.7 }}>
        <Sigma size={12} /> {showMaths ? 'hide' : 'why this film?'}
      </button>
    </div>
  )
}

/** Debug panel: why the engine picked this card, and what came close. */
function Maths({ model }: { model: Model }) {
  const pct = (x: number) => `${Math.round(x * 100)}%`
  return (
    <div className="rounded-xl p-3 font-mono text-[10px] leading-relaxed" style={{ background: 'var(--ink-deep)', color: 'var(--cream)' }}>
      <p className="opacity-60 mb-1">score = P(seen) × (unsure + new language + likes) × divisive</p>
      <table className="w-full">
        <thead className="opacity-60">
          <tr>
            <th className="text-left font-normal">film</th>
            <th className="text-right font-normal">P(like)</th>
            <th className="text-right font-normal">unsure</th>
            <th className="text-right font-normal">P(seen)</th>
            <th className="text-right font-normal">split</th>
            <th className="text-right font-normal">score</th>
          </tr>
        </thead>
        <tbody>
          {model.predictions.slice(0, 5).map((p, i) => (
            <tr key={p.film.tmdbId} style={{ opacity: i === 0 ? 1 : 0.65 }}>
              <td className="truncate max-w-[110px]">
                {i === 0 ? '▸ ' : '  '}
                {p.film.title}
              </td>
              <td className="text-right">{pct(p.pLike)}</td>
              <td className="text-right">{pct(p.uncertainty)}</td>
              <td className="text-right">{pct(p.pSeen)}</td>
              <td className="text-right">{p.film.divisive.toFixed(1)}</td>
              <td className="text-right">{p.score.toFixed(2)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function AnswerButton({ icon, label, onClick, bg, fg = 'white' }: { icon: React.ReactNode; label: string; onClick: () => void; bg: string; fg?: string }) {
  return (
    <motion.button
      whileTap={{ scale: 0.94 }}
      onClick={onClick}
      className="flex flex-col items-center gap-1 rounded-2xl py-2.5 font-sans text-[11px] font-semibold"
      style={{ background: bg, color: fg }}
    >
      {icon}
      {label}
    </motion.button>
  )
}

/* ── Reveal ─────────────────────────────────────────────────────────────── */

function Reveal({
  pool,
  state,
  submitError,
  onRetry,
  onMore,
  onNext,
}: {
  pool: Pool
  state: QuizState
  submitError: string | null
  onRetry: () => void
  onMore: () => void
  onNext: () => void
}) {
  const model = useMemo(() => fitModel(pool, state), [pool, state])
  const genres = genreAffinity(model)
  const into = genres.filter((g) => g.affinity > 0.1).slice(0, 3)
  const notInto = genres.filter((g) => g.affinity < -0.1).slice(-2).reverse()
  const picks = [...model.predictions].sort((a, b) => b.film.ratingMean + b.mean - (a.film.ratingMean + a.mean)).slice(0, 3)
  const rated = ratedCount(state)

  return (
    <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col flex-1 gap-6">
      <Header kicker="Your taste so far" title={rated >= 20 ? 'A clear picture' : 'A first sketch'} sub={`From ${rated} films. It sharpens as you rate more.`} />

      <div className="rounded-2xl p-4 flex flex-col gap-2 font-sans text-sm" style={{ background: 'var(--cream-2)' }}>
        {into.length > 0 && (
          <p>
            <span className="font-semibold" style={{ color: 'var(--teal)' }}>Drawn to</span> {into.map((g) => g.genre.toLowerCase()).join(', ')}
          </p>
        )}
        {notInto.length > 0 && (
          <p>
            <span className="font-semibold" style={{ color: 'var(--coral)' }}>Less into</span> {notInto.map((g) => g.genre.toLowerCase()).join(', ')}
          </p>
        )}
        {into.length === 0 && notInto.length === 0 && <p>Eclectic: no genre stands out yet.</p>}
      </div>

      <section className="flex flex-col gap-2">
        <p className="chapter-kicker">We think you&apos;d love</p>
        <div className="grid grid-cols-3 gap-2">
          {picks.map((p) => (
            <div key={p.film.tmdbId} className="flex flex-col gap-1">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={posterUrl(p.film.posterPath, 'w342')} alt={p.film.title} className="rounded-lg w-full object-cover" style={{ aspectRatio: '2/3' }} />
              <p className="font-sans text-[10px] leading-tight truncate" style={{ opacity: 0.7 }}>
                {p.film.title}
              </p>
            </div>
          ))}
        </div>
      </section>

      {submitError && (
        <div className="rounded-xl p-3 font-sans text-xs flex items-center justify-between gap-3" style={{ background: 'var(--coral-deep)', color: 'var(--cream)' }}>
          <span>Couldn&apos;t save your ratings. {submitError}</span>
          <button onClick={onRetry} className="font-semibold underline">
            Retry
          </button>
        </div>
      )}

      <div className="mt-auto flex flex-col gap-3">
        <Button onClick={onNext}>Find my matches</Button>
        <button onClick={onMore} className="font-sans text-sm font-semibold" style={{ color: 'var(--coral)' }}>
          Rate 10 more for sharper matches
        </button>
      </div>
    </motion.div>
  )
}

/* ── Basics ─────────────────────────────────────────────────────────────── */

const SEEKING = [
  { label: 'Women', value: ['women'] },
  { label: 'Men', value: ['men'] },
  { label: 'Everyone', value: ['women', 'men', 'nonbinary'] },
]

function Basics({
  ready,
  onDone,
}: {
  ready: boolean
  onDone: (b: { name: string; age: number; seeking: string[]; city: string }) => Promise<void>
}) {
  const [name, setName] = useState('')
  const [age, setAge] = useState(25)
  const [seeking, setSeeking] = useState<string[] | null>(null)
  const [city, setCity] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const go = async () => {
    if (!seeking || !name.trim()) return
    setSaving(true)
    setError(null)
    try {
      await onDone({ name: name.trim(), age, seeking, city })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setSaving(false)
    }
  }

  return (
    <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col flex-1 gap-7">
      <Header kicker="Last thing" title="About you" />

      <div className="flex flex-col gap-2">
        <p className="font-sans text-xs font-semibold uppercase tracking-[0.12em]" style={{ opacity: 0.5 }}>
          First name
        </p>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="What should matches call you?"
          maxLength={40}
          autoComplete="given-name"
          className="rounded-full px-5 py-3 font-sans text-sm outline-none"
          style={{ background: 'var(--cream-2)' }}
        />
      </div>

      <div className="flex flex-col gap-2">
        <p className="font-sans text-xs font-semibold uppercase tracking-[0.12em]" style={{ opacity: 0.5 }}>
          Show me
        </p>
        <div className="grid grid-cols-3 gap-2">
          {SEEKING.map((o) => {
            const on = seeking?.join() === o.value.join()
            return (
              <button
                key={o.label}
                onClick={() => setSeeking(o.value)}
                className="rounded-full py-3 font-sans text-sm font-semibold transition-colors"
                style={{ background: on ? 'var(--coral)' : 'var(--cream-2)', color: on ? 'white' : 'var(--ink-deep)' }}
              >
                {o.label}
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <p className="font-sans text-xs font-semibold uppercase tracking-[0.12em]" style={{ opacity: 0.5 }}>
          Your age
        </p>
        <div className="flex items-center gap-4">
          <button onClick={() => setAge((a) => Math.max(18, a - 1))} className="w-12 h-12 rounded-full font-sans text-xl" style={{ background: 'var(--cream-2)' }} aria-label="Younger">
            −
          </button>
          <span className="font-serif italic text-4xl w-16 text-center tabular-nums" style={{ color: 'var(--ink-deep)' }}>
            {age}
          </span>
          <button onClick={() => setAge((a) => Math.min(99, a + 1))} className="w-12 h-12 rounded-full font-sans text-xl" style={{ background: 'var(--cream-2)' }} aria-label="Older">
            +
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <p className="font-sans text-xs font-semibold uppercase tracking-[0.12em]" style={{ opacity: 0.5 }}>
          City <span className="normal-case tracking-normal font-normal">(optional)</span>
        </p>
        <input
          value={city}
          onChange={(e) => setCity(e.target.value)}
          placeholder="e.g. Berlin"
          maxLength={80}
          className="rounded-full px-5 py-3 font-sans text-sm outline-none"
          style={{ background: 'var(--cream-2)' }}
        />
      </div>

      {error && (
        <p className="font-sans text-xs" style={{ color: 'var(--coral)' }}>
          {error}
        </p>
      )}

      <div className="mt-auto">
        <Button onClick={go} disabled={!seeking || !name.trim() || saving || !ready}>
          {!ready ? 'Saving your ratings…' : saving ? 'One moment…' : 'See my matches'}
        </Button>
      </div>
    </motion.div>
  )
}
