/**
 * Decorative divider components matching the Framed design system.
 * Server components — purely presentational.
 */

/** Props for dividers that support className */
interface DividerProps {
  /** Additional CSS class names */
  className?: string
}

/** Double-rule decorative divider — coral-deep, opacity 0.4 */
export function RuleDouble({ className = '' }: DividerProps) {
  return (
    <div
      className={`w-full ${className}`}
      style={{
        borderTop: '2px double var(--coral-deep)',
        opacity: 0.4,
      }}
      role="separator"
      aria-hidden="true"
    />
  )
}

/** Single thin rule — coral-deep, opacity 0.2 */
export function RuleThin({ className = '' }: DividerProps) {
  return (
    <div
      className={`w-full ${className}`}
      style={{
        borderTop: '1px solid var(--coral-deep)',
        opacity: 0.2,
      }}
      role="separator"
      aria-hidden="true"
    />
  )
}

/** Diamond divider: thin rules either side of a centred ◆ */
export function DiamondDivider({ className = '' }: DividerProps) {
  return (
    <div
      className={`flex items-center gap-3 w-full ${className}`}
      role="separator"
      aria-hidden="true"
    >
      <div
        className="flex-1"
        style={{ borderTop: '1px solid var(--coral-deep)', opacity: 0.2 }}
      />
      <span
        className="text-xs font-sans"
        style={{ color: 'var(--coral-deep)', opacity: 0.4 }}
      >
        ◆
      </span>
      <div
        className="flex-1"
        style={{ borderTop: '1px solid var(--coral-deep)', opacity: 0.2 }}
      />
    </div>
  )
}
