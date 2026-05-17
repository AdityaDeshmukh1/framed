'use client'

/**
 * Onboarding page — Step 1/3.
 * Collects a Letterboxd username, starts the scrape pipeline, navigates to /portrait.
 */

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { motion } from 'framer-motion'
import { PhoneShell } from '../../components/layout/PhoneShell'
import { StatusBar } from '../../components/layout/StatusBar'
import { Input } from '../../components/ui/Input'
import { Button } from '../../components/ui/Button'
import { RuleDouble } from '../../components/ui/Dividers'
import { startOnboarding } from '../../lib/api'
import { MOCK_USER_ID } from '../../lib/constants'

export default function OnboardingPage() {
  const router = useRouter()
  const [handle, setHandle] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const trimmedHandle = handle.trim().replace(/^@/, '')
  const isValid = trimmedHandle.length >= 2

  async function handleSubmit() {
    if (!isValid || isLoading) return
    setIsLoading(true)
    setError(null)

    try {
      const data = await startOnboarding(MOCK_USER_ID, trimmedHandle)
      router.push(`/portrait?jobId=${data.job_id}`)
    } catch {
      // If the backend is not running, proceed to portrait with a mock job ID
      router.push(`/portrait?jobId=mock-job-001`)
    } finally {
      setIsLoading(false)
    }
  }

  function handleSkip() {
    router.push('/portrait?jobId=mock-job-001')
  }

  return (
    <PhoneShell>
      {/* Status bar */}
      <StatusBar theme="light" />

      {/* Progress dots */}
      <div className="flex items-center justify-center gap-2 py-3">
        {[0, 1, 2].map((i) => (
          <div
            key={i}
            className="rounded-full transition-all"
            style={{
              width: i === 0 ? '20px' : '6px',
              height: '6px',
              background: i === 0 ? 'var(--coral)' : 'var(--coral-deep)',
              opacity: i === 0 ? 1 : 0.2,
            }}
          />
        ))}
      </div>

      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: 'easeOut' }}
        className="flex flex-col flex-1 px-6 pt-4 pb-8 gap-8"
      >
        {/* Chapter header */}
        <div className="flex flex-col gap-3">
          <RuleDouble />
          <p className="chapter-kicker">Chapter I</p>
          <div className="flex flex-col gap-1">
            <h1
              className="font-serif italic text-4xl leading-none"
              style={{ color: 'var(--ink-deep)' }}
            >
              Your Film History
            </h1>
            <p
              className="font-sans text-sm"
              style={{ color: 'var(--ink)', opacity: 0.6 }}
            >
              Let&apos;s find out who you are
            </p>
          </div>
          <RuleDouble />
        </div>

        {/* Input section */}
        <div className="flex flex-col gap-3">
          <label
            htmlFor="letterboxd-handle"
            className="font-sans text-xs font-semibold uppercase tracking-[0.12em]"
            style={{ color: 'var(--ink)', opacity: 0.5 }}
          >
            Your Letterboxd username
          </label>

          <Input
            prefix="letterboxd.com/"
            value={handle}
            onChange={setHandle}
            placeholder="your-username"
            autoFocus
            disabled={isLoading}
          />

          {/* Hint */}
          <p
            className="font-sans text-xs leading-relaxed"
            style={{ color: 'var(--ink)', opacity: 0.45 }}
          >
            We read your public watchlist and ratings — nothing is stored beyond your taste fingerprint.
          </p>
        </div>

        {/* Example block */}
        <div
          className="rounded-xl p-4"
          style={{ background: 'var(--cream-2)' }}
        >
          <p
            className="font-sans text-[10px] uppercase tracking-widest mb-2 font-semibold"
            style={{ color: 'var(--ink)', opacity: 0.4 }}
          >
            Example
          </p>
          <p
            className="font-mono text-sm"
            style={{ color: 'var(--coral-deep)' }}
          >
            letterboxd.com/<span style={{ color: 'var(--coral)' }}>bergmanfan1992</span>
          </p>
        </div>

        {error && (
          <p
            className="font-sans text-sm text-center"
            style={{ color: 'var(--coral)' }}
          >
            {error}
          </p>
        )}

        {/* CTAs */}
        <div className="flex flex-col gap-3 mt-auto">
          <Button
            variant="primary"
            onClick={handleSubmit}
            disabled={!isValid || isLoading}
          >
            {isLoading ? 'Starting import…' : 'Import my films'}
          </Button>
          <button
            type="button"
            onClick={handleSkip}
            className="font-sans text-sm text-center py-2 transition-opacity hover:opacity-70"
            style={{ color: 'var(--ink)', opacity: 0.45 }}
          >
            I&apos;ll do this later
          </button>
        </div>
      </motion.div>
    </PhoneShell>
  )
}
