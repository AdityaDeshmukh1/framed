/**
 * PhoneShell — max-width 390px wrapper that centres content like a phone viewport.
 * Server component — purely structural.
 */

/** Props for the PhoneShell component */
interface Props {
  /** Page content */
  children: React.ReactNode
  /** Additional CSS class names */
  className?: string
  /** Whether to add bottom padding for the BottomNav (default false) */
  hasBottomNav?: boolean
  /** Background color override */
  background?: string
}

export function PhoneShell({
  children,
  className = '',
  hasBottomNav = false,
  background,
}: Props) {
  return (
    <div
      className="min-h-screen flex justify-center"
      style={{ background: background ?? 'var(--cream)' }}
    >
      <div
        className={`relative w-full max-w-[390px] min-h-screen flex flex-col overflow-x-hidden ${hasBottomNav ? 'pb-20' : ''} ${className}`}
        style={{ background: background ?? 'var(--cream)' }}
      >
        {children}
      </div>
    </div>
  )
}
