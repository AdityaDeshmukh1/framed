'use client'

/**
 * AppBar — top navigation bar with optional back button, title, and action slot.
 */

import { useRouter } from 'next/navigation'
import { Button } from '../ui/Button'

/** Props for the AppBar component */
interface Props {
  /** Whether to show a back button on the left */
  showBack?: boolean
  /** Centre title text */
  title?: string
  /** Whether to render the serif italic "Framed" wordmark instead of title text */
  showLogo?: boolean
  /** Right-side action slot (e.g. settings icon) */
  rightAction?: React.ReactNode
  /** Override the back navigation URL instead of using browser history */
  backHref?: string
}

export function AppBar({
  showBack = false,
  title,
  showLogo = false,
  rightAction,
  backHref,
}: Props) {
  const router = useRouter()

  function handleBack() {
    if (backHref) {
      router.push(backHref)
    } else {
      router.back()
    }
  }

  return (
    <div
      className="flex items-center justify-between px-4 py-3 sticky top-0 z-10"
      style={{ background: 'var(--cream)', borderBottom: '1px solid rgba(74,27,12,0.1)' }}
    >
      {/* Left: back button or spacer */}
      <div className="w-10">
        {showBack && (
          <Button variant="icon" onClick={handleBack} aria-label="Go back">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12.5 5L7.5 10L12.5 15" />
            </svg>
          </Button>
        )}
      </div>

      {/* Centre: logo or title */}
      <div className="flex-1 text-center">
        {showLogo ? (
          <span
            className="font-serif italic text-xl tracking-wider"
            style={{ color: 'var(--coral-deep)' }}
          >
            Framed
          </span>
        ) : title ? (
          <span
            className="font-sans text-sm font-semibold tracking-wide"
            style={{ color: 'var(--ink-deep)' }}
          >
            {title}
          </span>
        ) : null}
      </div>

      {/* Right: action slot or spacer */}
      <div className="w-10 flex justify-end">
        {rightAction}
      </div>
    </div>
  )
}
