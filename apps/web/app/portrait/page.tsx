'use client'

/**
 * Portrait page — 3-phase reveal.
 *
 * Phase 1 "loading":    polls /api/status/[jobId] showing pipeline progress.
 * Phase 2 "generating": pipeline complete, calling POST /soul to write the portrait.
 * Phase 3 "reveal":     animated reveal of the real soul portrait.
 *
 * useSearchParams() is isolated in PortraitInner and wrapped in <Suspense>
 * to satisfy Next.js static generation requirements.
 */

import { Suspense, useEffect, useState, useCallback } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { motion, AnimatePresence } from 'framer-motion'
import { PhoneShell } from '../../components/layout/PhoneShell'
import { StatusBar } from '../../components/layout/StatusBar'
import { Button } from '../../components/ui/Button'
import { PosterGrid } from '../../components/features/PosterGrid'
import { SoulPortrait } from '../../components/features/SoulPortrait'
import { RuleDouble, DiamondDivider } from '../../components/ui/Dividers'
import { fetchJobStatus, generateSoul, computeMatches } from '../../lib/api'
import { MOCK_SOUL_PORTRAIT, MOCK_USER, MOCK_USER_ID } from '../../lib/constants'
import type { ScrapeJob } from '../../lib/types'

type Phase = 'loading' | 'generating' | 'reveal'

const PIPELINE_STAGES = [
  { key: 'scraping',  label: 'Scanning Letterboxd',  numeral: 'I',  doneStatuses: ['enriching', 'embedding', 'complete'] },
  { key: 'enriching', label: 'Enriching metadata',   numeral: 'II', doneStatuses: ['embedding', 'complete'] },
  { key: 'embedding', label: 'Computing embeddings', numeral: 'III',doneStatuses: ['complete'] },
  { key: 'writing',   label: 'Writing your soul',    numeral: 'IV', doneStatuses: [] as string[] },
]

const ACCURACY_OPTIONS = [
  { label: 'nailed it',    value: 5 },
  { label: 'mostly right', value: 4 },
  { label: 'bit off',      value: 2 },
  { label: 'who is this?', value: 1 },
]

const MOCK_FILMS = [
  { id: 'f1', title: 'In the Mood for Love', year: 2000 },
  { id: 'f2', title: 'Stalker',              year: 1979 },
  { id: 'f3', title: 'Chungking Express',    year: 1994 },
  { id: 'f4', title: 'Persona',              year: 1966 },
]

function stageProgress(job: ScrapeJob | null, phase: Phase): number {
  if (phase === 'reveal')     return 100
  if (phase === 'generating') return 90
  if (!job) return 0
  switch (job.status) {
    case 'queued':    return 5
    case 'scraping':  return 25
    case 'enriching': return 50
    case 'embedding': return 75
    case 'complete':  return 85
    default:          return 0
  }
}

function isStageComplete(stageKey: string, job: ScrapeJob | null, phase: Phase): boolean {
  if (stageKey === 'writing') return phase === 'reveal'
  if (!job) return false
  const stage = PIPELINE_STAGES.find((s) => s.key === stageKey)
  return stage?.doneStatuses.includes(job.status) ?? false
}

function isStageActive(stageKey: string, job: ScrapeJob | null, phase: Phase): boolean {
  if (stageKey === 'writing') return phase === 'generating'
  if (!job) return false
  const done = isStageComplete(stageKey, job, phase)
  return job.status === stageKey && !done
}

function PortraitInner() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const jobId = searchParams.get('jobId')

  const [job, setJob] = useState<ScrapeJob | null>(null)
  const [phase, setPhase] = useState<Phase>('loading')
  const [portrait, setPortrait] = useState<string>(MOCK_SOUL_PORTRAIT.portrait)
  const [selectedAccuracy, setSelectedAccuracy] = useState<number | null>(null)

  // Called once the pipeline job reaches 'complete'.
  const onPipelineComplete = useCallback(async (isMock: boolean) => {
    setPhase('generating')
    console.log('[framed:portrait] pipeline complete — generating soul portrait')

    if (isMock) {
      // Simulate generation delay in mock mode
      await new Promise((r) => setTimeout(r, 1500))
      setPortrait(MOCK_SOUL_PORTRAIT.portrait)
    } else {
      try {
        const data = await generateSoul(MOCK_USER_ID)
        setPortrait(data.portrait)
        // Fire-and-forget: compute matches in the background
        computeMatches(MOCK_USER_ID).catch((err) =>
          console.warn('[framed:portrait] match computation failed (non-critical):', err)
        )
      } catch (err) {
        console.warn('[framed:portrait] soul generation failed, using mock:', err)
        setPortrait(MOCK_SOUL_PORTRAIT.portrait)
      }
    }

    console.log('[framed:portrait] portrait ready — transitioning to reveal')
    setTimeout(() => setPhase('reveal'), 400)
  }, [])

  const pollStatus = useCallback(async () => {
    if (!jobId || jobId === 'mock-job-001') {
      // Mock simulation: cycle through pipeline statuses
      const statuses: ScrapeJob['status'][] = ['queued', 'scraping', 'enriching', 'embedding', 'complete']
      let i = 0
      const interval = setInterval(() => {
        const status = statuses[Math.min(i, statuses.length - 1)]
        console.log(`[framed:portrait] mock status → ${status}`)
        setJob({
          id: 'mock-job-001',
          status,
          films_found:     status === 'queued' ? 0 : 847,
          films_processed: status === 'complete' ? 847 : i * 200,
          started_at:      new Date().toISOString(),
          completed_at:    status === 'complete' ? new Date().toISOString() : null,
        })
        if (status === 'complete') {
          clearInterval(interval)
          setTimeout(() => onPipelineComplete(true), 600)
        }
        i++
      }, 900)
      return () => clearInterval(interval)
    }

    // Real polling loop
    try {
      const data = await fetchJobStatus(jobId)
      console.log(`[framed:portrait] status=${data.status}  films=${data.films_processed}/${data.films_found}`)
      setJob(data)
      if (data.status === 'complete') {
        await onPipelineComplete(false)
      } else if (data.status === 'failed') {
        console.error('[framed:portrait] job failed — falling back to mock')
        await onPipelineComplete(true)
      } else {
        setTimeout(pollStatus, 2000)
      }
    } catch (err) {
      console.warn('[framed:portrait] status fetch error (backend down?):', err)
      // Backend not running — skip straight to mock reveal
      await onPipelineComplete(true)
    }
  }, [jobId, onPipelineComplete])

  useEffect(() => {
    pollStatus()
  }, [pollStatus])

  const progress = stageProgress(job, phase)

  return (
    <PhoneShell>
      <StatusBar theme="light" />

      <AnimatePresence mode="wait">
        {phase !== 'reveal' ? (
          /* ── Loading + Generating phase ── */
          <motion.div
            key="loading"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="flex flex-col flex-1 px-6 pt-4 pb-8 gap-6"
          >
            <div className="flex flex-col gap-3">
              <RuleDouble />
              <p className="chapter-kicker">Processing</p>
              <h1 className="font-serif italic text-3xl leading-tight" style={{ color: 'var(--ink-deep)' }}>
                Building your portrait
              </h1>
              <RuleDouble />
            </div>

            {/* Progress bar */}
            <div className="w-full h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--cream-2)' }}>
              <motion.div
                className="h-full rounded-full"
                style={{ background: 'var(--coral)' }}
                animate={{ width: `${progress}%` }}
                transition={{ duration: 0.5, ease: 'easeOut' }}
              />
            </div>

            {/* Pipeline stage cards */}
            <div className="flex flex-col gap-3">
              {PIPELINE_STAGES.map((stage, idx) => {
                const done   = isStageComplete(stage.key, job, phase)
                const active = isStageActive(stage.key, job, phase)

                return (
                  <motion.div
                    key={stage.key}
                    initial={{ opacity: 0, y: 8 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: idx * 0.1 }}
                    className="flex items-center gap-4 p-4 rounded-xl"
                    style={{ background: 'var(--cream-2)', opacity: done ? 1 : active ? 1 : 0.5 }}
                  >
                    <span
                      className="font-serif italic text-2xl w-6 text-center flex-shrink-0"
                      style={{ color: 'var(--coral-deep)', opacity: 0.6 }}
                    >
                      {stage.numeral}
                    </span>

                    <div className="flex flex-col flex-1 gap-0.5">
                      <span className="font-sans text-sm font-medium" style={{ color: 'var(--ink-deep)' }}>
                        {stage.label}
                      </span>
                      {job && (done || active) && stage.key !== 'writing' && (
                        <span className="font-mono text-[10px] tabular-nums" style={{ color: 'var(--ink)', opacity: 0.45 }}>
                          {job.films_processed} films
                        </span>
                      )}
                    </div>

                    {done ? (
                      <span style={{ color: 'var(--teal)' }}>✓</span>
                    ) : active ? (
                      <motion.div
                        className="w-4 h-4 rounded-full border-2"
                        style={{ borderColor: 'var(--coral)', borderTopColor: 'transparent' }}
                        animate={{ rotate: 360 }}
                        transition={{ duration: 0.8, repeat: Infinity, ease: 'linear' }}
                      />
                    ) : (
                      <div className="w-4 h-4 rounded-full border-2" style={{ borderColor: 'var(--ink)', opacity: 0.15 }} />
                    )}
                  </motion.div>
                )
              })}
            </div>

            <p className="font-sans text-xs text-center" style={{ color: 'var(--ink)', opacity: 0.35 }}>
              This takes about 30 seconds for a large watchlist
            </p>
          </motion.div>
        ) : (
          /* ── Reveal phase ── */
          <motion.div
            key="reveal"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, ease: 'easeOut' }}
            className="flex flex-col flex-1 px-6 pt-4 pb-8 gap-6"
          >
            {/* Identity header */}
            <div className="flex flex-col gap-2">
              <RuleDouble />
              <p className="chapter-kicker">Your Portrait</p>
              <div className="flex items-baseline gap-3">
                <h1 className="font-serif italic text-4xl" style={{ color: 'var(--ink-deep)' }}>
                  {MOCK_USER.display_name}
                </h1>
                <span className="font-mono text-sm" style={{ color: 'var(--ink)', opacity: 0.45 }}>
                  {MOCK_USER.age} · {MOCK_USER.location}
                </span>
              </div>
              <RuleDouble />
            </div>

            {/* Real soul portrait text */}
            <SoulPortrait portrait={portrait} showDividers={false} />

            <DiamondDivider />

            {/* Opening titles */}
            <div className="flex flex-col gap-3">
              <p className="chapter-kicker">Opening Titles</p>
              <PosterGrid films={MOCK_FILMS} />
            </div>

            <DiamondDivider />

            {/* Accuracy feedback */}
            <div className="flex flex-col gap-3">
              <p className="font-sans text-sm font-medium text-center" style={{ color: 'var(--ink)', opacity: 0.7 }}>
                How accurate is this?
              </p>
              <div className="grid grid-cols-2 gap-2">
                {ACCURACY_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setSelectedAccuracy(opt.value)}
                    className="py-3 px-4 rounded-xl font-sans text-sm font-medium transition-all"
                    style={{
                      background: selectedAccuracy === opt.value ? 'var(--coral)' : 'var(--cream-2)',
                      color:      selectedAccuracy === opt.value ? 'white' : 'var(--ink)',
                      opacity:    selectedAccuracy !== null && selectedAccuracy !== opt.value ? 0.5 : 1,
                    }}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="mt-auto">
              <Button variant="primary" onClick={() => router.push('/discover')}>
                Enter the cinema
              </Button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </PhoneShell>
  )
}

export default function PortraitPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center" style={{ background: 'var(--cream)' }}>
        <span className="font-serif italic text-lg" style={{ color: 'var(--coral)', opacity: 0.6 }}>Loading…</span>
      </div>
    }>
      <PortraitInner />
    </Suspense>
  )
}
