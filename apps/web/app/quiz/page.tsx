'use client'

/**
 * Rating quiz — MOCKUP. Builds a taste profile for people without a Letterboxd
 * history (or with a thin one). Fully client-side with a hardcoded film pool;
 * nothing is sent to the backend yet. The final screen shows the payload a
 * `quiz` importer would receive.
 *
 * Steps: intro → pick 4 favourites → rate ~20 films (swipe) → profile.
 * Keys while rating: → loved, ← nope, ↑ fine, ↓ haven't seen, Z undo.
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Heart, Meh, RotateCcw, ThumbsDown, EyeOff, Search, Check } from 'lucide-react'
import { PhoneShell } from '../../components/layout/PhoneShell'
import { Button } from '../../components/ui/Button'
import { RuleDouble } from '../../components/ui/Dividers'
import { QuizCard, posterColor } from '../../components/features/QuizCard'
import { POSTER_COLORS } from '../../lib/constants'
import { FACET_LABELS, QUIZ_FILMS } from '../../lib/quiz/films'
import {
  type Answer,
  type QuizState,
  MIN_RATED,
  TARGET_RATED,
  buildProfile,
  filmById,
  isDone,
  nextFilm,
  ratedCount,
  toImportPayload,
} from '../../lib/quiz/engine'

type Step = 'intro' | 'favourites' | 'rate' | 'result'

const EMPTY: QuizState = { favourites: [], responses: [] }

export default function QuizPage() {
  const [step, setStep] = useState<Step>('intro')
  const [state, setState] = useState<QuizState>(EMPTY)

  const restart = () => {
    setState(EMPTY)
    setStep('intro')
  }

  return (
    <PhoneShell>
      <div className="flex flex-col flex-1 px-6 pt-8 pb-8">
        {step === 'intro' && <Intro onStart={() => setStep('favourites')} />}
        {step === 'favourites' && (
          <Favourites
            selected={state.favourites}
            onChange={(favourites) => setState((s) => ({ ...s, favourites }))}
            onNext={() => setStep('rate')}
          />
        )}
        {step === 'rate' && (
          <Rate state={state} setState={setState} onDone={() => setStep('result')} />
        )}
        {step === 'result' && <Result state={state} onRestart={restart} onMore={() => setStep('rate')} />}
      </div>
    </PhoneShell>
  )
}

/* ── Header ─────────────────────────────────────────────────────────────── */

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

/* ── Intro ──────────────────────────────────────────────────────────────── */

function Intro({ onStart }: { onStart: () => void }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex flex-col flex-1 gap-8"
    >
      <Header kicker="No Letterboxd? No problem" title="The Screen Test" sub="About a minute. No wrong answers." />
      <ol className="flex flex-col gap-4 font-sans text-sm">
        {[
          ['Pick four favourites', 'The films that say the most about you.'],
          ['Rate about twenty more', 'Swipe right if you loved it, left if not. Skip anything you haven’t seen.'],
          ['See your profile', 'We pick each film to learn the most about your taste.'],
        ].map(([t, d], i) => (
          <li key={t} className="flex gap-4">
            <span className="font-serif italic text-3xl leading-none" style={{ color: 'var(--coral)' }}>
              {i + 1}
            </span>
            <div>
              <p className="font-semibold" style={{ color: 'var(--ink-deep)' }}>
                {t}
              </p>
              <p style={{ opacity: 0.6 }}>{d}</p>
            </div>
          </li>
        ))}
      </ol>
      <div className="mt-auto">
        <Button onClick={onStart}>Start</Button>
      </div>
    </motion.div>
  )
}

/* ── Favourites ─────────────────────────────────────────────────────────── */

function Favourites({
  selected,
  onChange,
  onNext,
}: {
  selected: string[]
  onChange: (ids: string[]) => void
  onNext: () => void
}) {
  const [query, setQuery] = useState('')
  const films = useMemo(() => {
    const q = query.trim().toLowerCase()
    const list = q
      ? QUIZ_FILMS.filter((f) => f.title.toLowerCase().includes(q) || f.director.toLowerCase().includes(q))
      : [...QUIZ_FILMS].sort((a, b) => b.popularity - a.popularity)
    return list
  }, [query])

  const toggle = (id: string) => {
    if (selected.includes(id)) onChange(selected.filter((x) => x !== id))
    else if (selected.length < 4) onChange([...selected, id])
  }

  return (
    <div className="flex flex-col flex-1 gap-5 min-h-0">
      <Header kicker="Step 1 of 2" title="Four Favourites" sub="Pick up to four. You can skip this." />

      {/* Selected slots */}
      <div className="grid grid-cols-4 gap-2">
        {[0, 1, 2, 3].map((i) => {
          const film = selected[i] ? filmById.get(selected[i]) : undefined
          const colors = film ? POSTER_COLORS[posterColor(film.id)] : null
          return (
            <button
              key={i}
              onClick={() => film && toggle(film.id)}
              className="rounded-lg flex items-end p-1.5 text-left"
              style={{
                aspectRatio: '2/3',
                background: colors?.bg ?? 'transparent',
                color: colors?.text,
                border: film ? 'none' : '1.5px dashed rgba(74,27,12,0.25)',
              }}
              aria-label={film ? `Remove ${film.title}` : `Empty slot ${i + 1}`}
            >
              {film && <span className="font-serif italic text-[11px] leading-tight font-semibold">{film.title}</span>}
            </button>
          )
        })}
      </div>

      <label className="flex items-center gap-2 rounded-full px-4 py-3" style={{ background: 'var(--cream-2)' }}>
        <Search size={16} style={{ opacity: 0.5 }} />
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search films or directors"
          className="bg-transparent outline-none flex-1 font-sans text-sm"
        />
      </label>

      <div className="grid grid-cols-3 gap-2 overflow-y-auto max-h-[38vh] pr-1">
        {films.map((f) => {
          const isOn = selected.includes(f.id)
          const { bg, text } = POSTER_COLORS[posterColor(f.id)]
          return (
            <button
              key={f.id}
              onClick={() => toggle(f.id)}
              disabled={!isOn && selected.length >= 4}
              className="relative rounded-lg p-2 flex flex-col justify-between text-left transition-opacity disabled:opacity-40"
              style={{ aspectRatio: '2/3', background: bg, color: text, outline: isOn ? '3px solid var(--ink-deep)' : 'none', outlineOffset: 2 }}
            >
              <span className="font-mono text-[9px] opacity-70">{f.year}</span>
              <span className="font-serif italic text-xs leading-tight font-semibold">{f.title}</span>
              {isOn && (
                <span className="absolute top-1.5 right-1.5 rounded-full p-0.5" style={{ background: 'var(--ink-deep)' }}>
                  <Check size={12} color="white" />
                </span>
              )}
            </button>
          )
        })}
        {films.length === 0 && (
          <p className="col-span-3 text-center font-sans text-sm py-6" style={{ opacity: 0.5 }}>
            Not in the mock pool. The real version will search all of TMDB.
          </p>
        )}
      </div>

      <div className="mt-auto pt-2">
        <Button onClick={onNext}>{selected.length ? `Continue with ${selected.length}` : 'Skip'}</Button>
      </div>
    </div>
  )
}

/* ── Rate ───────────────────────────────────────────────────────────────── */

function Rate({
  state,
  setState,
  onDone,
}: {
  state: QuizState
  setState: React.Dispatch<React.SetStateAction<QuizState>>
  onDone: () => void
}) {
  const film = nextFilm(state)
  const rated = ratedCount(state)
  const profile = buildProfile(state)
  const [exitDir, setExitDir] = useState<'left' | 'right' | 'up' | 'down'>('right')

  const answer = useCallback(
    (a: Answer) => {
      if (!film) return
      setExitDir(a === 'loved' ? 'right' : a === 'nope' ? 'left' : a === 'fine' ? 'up' : 'down')
      const next = { ...state, responses: [...state.responses, { filmId: film.id, answer: a }] }
      setState(next)
      if (isDone(next)) setTimeout(onDone, 250)
    },
    [film, state, setState, onDone],
  )

  const undo = useCallback(() => {
    setState((s) => ({ ...s, responses: s.responses.slice(0, -1) }))
  }, [setState])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const map: Record<string, Answer> = { ArrowRight: 'loved', ArrowLeft: 'nope', ArrowUp: 'fine', ArrowDown: 'unseen' }
      if (map[e.key]) {
        e.preventDefault()
        answer(map[e.key])
      } else if (e.key === 'z' || e.key === 'Backspace') undo()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [answer, undo])

  const exit = {
    right: { x: 400, rotate: 15, opacity: 0 },
    left: { x: -400, rotate: -15, opacity: 0 },
    up: { y: -300, opacity: 0 },
    down: { y: 300, opacity: 0, scale: 0.9 },
  }[exitDir]

  return (
    <div className="flex flex-col flex-1 gap-5">
      {/* Progress + strength */}
      <div className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between font-sans">
          <p className="chapter-kicker">Step 2 of 2</p>
          <p className="text-xs tabular-nums" style={{ opacity: 0.6 }}>
            {rated}/{TARGET_RATED} rated
          </p>
        </div>
        <StrengthBar strength={profile.strength} label={profile.strengthLabel} />
      </div>

      {/* Card stack */}
      <div className="relative flex-1 min-h-[360px]">
        <div className="absolute inset-x-4 inset-y-0 top-3 rounded-3xl" style={{ background: 'var(--cream-2)' }} />
        <AnimatePresence custom={exitDir}>
          {film && (
            <motion.div
              key={film.id}
              className="absolute inset-0"
              initial={{ scale: 0.95, opacity: 0, y: 10 }}
              animate={{ scale: 1, opacity: 1, y: 0 }}
              exit={exit}
              transition={{ duration: 0.25 }}
            >
              <QuizCard film={film} onSwipe={(d) => answer(d === 'right' ? 'loved' : 'nope')} />
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Buttons */}
      <div className="grid grid-cols-3 gap-3">
        <AnswerButton icon={<ThumbsDown size={22} />} label="Not for me" onClick={() => answer('nope')} bg="var(--ink-deep)" />
        <AnswerButton icon={<Meh size={22} />} label="It's fine" onClick={() => answer('fine')} bg="var(--amber)" fg="var(--ink-deep)" />
        <AnswerButton icon={<Heart size={22} />} label="Loved it" onClick={() => answer('loved')} bg="var(--teal)" />
      </div>
      <div className="flex items-center justify-between font-sans text-sm">
        <button onClick={undo} disabled={!state.responses.length} className="flex items-center gap-1.5 disabled:opacity-30" style={{ opacity: 0.7 }}>
          <RotateCcw size={14} /> Undo
        </button>
        <button onClick={() => answer('unseen')} className="flex items-center gap-1.5 font-semibold" style={{ color: 'var(--coral)' }}>
          <EyeOff size={14} /> Haven&apos;t seen it
        </button>
        <button onClick={onDone} disabled={rated < MIN_RATED} className="disabled:opacity-30" style={{ opacity: 0.7 }}>
          Finish
        </button>
      </div>
      <p className="hidden md:block text-center font-mono text-[10px]" style={{ opacity: 0.4 }}>
        ← nope · ↑ fine · → loved · ↓ haven&apos;t seen · Z undo
      </p>
    </div>
  )
}

function AnswerButton({
  icon,
  label,
  onClick,
  bg,
  fg = 'white',
}: {
  icon: React.ReactNode
  label: string
  onClick: () => void
  bg: string
  fg?: string
}) {
  return (
    <motion.button
      whileTap={{ scale: 0.94 }}
      onClick={onClick}
      className="flex flex-col items-center gap-1 rounded-2xl py-3 font-sans text-xs font-semibold"
      style={{ background: bg, color: fg }}
    >
      {icon}
      {label}
    </motion.button>
  )
}

function StrengthBar({ strength, label }: { strength: number; label: string }) {
  return (
    <div className="flex items-center gap-3">
      <div className="flex-1 h-2 rounded-full overflow-hidden" style={{ background: 'var(--cream-2)' }}>
        <motion.div
          className="h-full rounded-full"
          style={{ background: 'var(--coral)' }}
          animate={{ width: `${Math.round(strength * 100)}%` }}
          transition={{ duration: 0.4 }}
        />
      </div>
      <span className="font-sans text-xs font-semibold w-16 text-right" style={{ color: 'var(--coral-deep)' }}>
        {label}
      </span>
    </div>
  )
}

/* ── Result ─────────────────────────────────────────────────────────────── */

function Result({ state, onRestart, onMore }: { state: QuizState; onRestart: () => void; onMore: () => void }) {
  const profile = buildProfile(state)
  const [showPayload, setShowPayload] = useState(false)
  const loves = profile.facets.filter((f) => f.affinity > 0.15).slice(0, 3)
  const avoids = profile.facets.filter((f) => f.affinity < -0.15).slice(-3).reverse()

  return (
    <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col flex-1 gap-6">
      <Header
        kicker="Your profile"
        title={profile.strengthLabel === 'Portrait' ? 'A clear picture' : profile.strengthLabel === 'Outline' ? 'Taking shape' : 'A first sketch'}
        sub={`Built from ${state.favourites.length} favourites and ${profile.rated} ratings · ${profile.unseen} skipped`}
      />

      <StrengthBar strength={profile.strength} label={profile.strengthLabel} />

      {(loves.length > 0 || avoids.length > 0) && (
        <div className="rounded-2xl p-4 flex flex-col gap-2 font-sans text-sm" style={{ background: 'var(--cream-2)' }}>
          {loves.length > 0 && (
            <p>
              <span className="font-semibold" style={{ color: 'var(--teal)' }}>Drawn to</span>{' '}
              {loves.map((f) => FACET_LABELS[f.facet]).join(', ')}
            </p>
          )}
          {avoids.length > 0 && (
            <p>
              <span className="font-semibold" style={{ color: 'var(--coral)' }}>Less into</span>{' '}
              {avoids.map((f) => FACET_LABELS[f.facet]).join(', ')}
            </p>
          )}
        </div>
      )}

      {/* Facet bars: diverging from centre */}
      <div className="flex flex-col gap-2.5">
        {profile.facets.map((f) => (
          <div key={f.facet} className="grid grid-cols-[110px_1fr_24px] items-center gap-3 font-sans text-xs">
            <span style={{ color: 'var(--ink-deep)' }}>{FACET_LABELS[f.facet]}</span>
            <div className="relative h-2 rounded-full" style={{ background: 'var(--cream-2)' }}>
              <div className="absolute left-1/2 top-[-3px] bottom-[-3px] w-px" style={{ background: 'var(--ink)', opacity: 0.25 }} />
              <motion.div
                className="absolute top-0 bottom-0 rounded-full"
                style={{
                  background: f.affinity >= 0 ? 'var(--teal)' : 'var(--coral)',
                  left: f.affinity >= 0 ? '50%' : undefined,
                  right: f.affinity < 0 ? '50%' : undefined,
                }}
                initial={{ width: 0 }}
                animate={{ width: `${Math.abs(f.affinity) * 50}%` }}
                transition={{ duration: 0.5 }}
              />
            </div>
            <span className="tabular-nums text-right" style={{ opacity: 0.4 }} title="films rated in this facet">
              {f.evidence}
            </span>
          </div>
        ))}
      </div>

      <button onClick={() => setShowPayload((v) => !v)} className="font-mono text-xs text-left" style={{ color: 'var(--coral)' }}>
        {showPayload ? '▾' : '▸'} What the backend would receive
      </button>
      {showPayload && (
        <pre className="font-mono text-[10px] rounded-xl p-3 overflow-x-auto max-h-60" style={{ background: 'var(--ink-deep)', color: 'var(--cream)' }}>
          {JSON.stringify(toImportPayload(state), null, 2)}
        </pre>
      )}

      <div className="mt-auto flex flex-col gap-3">
        {!isDone(state) && <Button onClick={onMore}>Rate more to sharpen it</Button>}
        <Button variant="ghost" onClick={onRestart}>
          Start over
        </Button>
      </div>
    </motion.div>
  )
}
