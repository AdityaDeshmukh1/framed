'use client'

/**
 * Input component with optional prefix overlay (e.g. "letterboxd.com/").
 * Designed for the onboarding username entry.
 */

/** Props for the Input component */
interface Props {
  /** Visible prefix overlaid on the left of the input */
  prefix?: string
  /** Current value */
  value: string
  /** Change handler */
  onChange: (value: string) => void
  /** Placeholder text */
  placeholder?: string
  /** Input type */
  type?: string
  /** Whether the input is disabled */
  disabled?: boolean
  /** Additional CSS class names */
  className?: string
  /** Auto-focus on mount */
  autoFocus?: boolean
}

export function Input({
  prefix,
  value,
  onChange,
  placeholder = '',
  type = 'text',
  disabled = false,
  className = '',
  autoFocus = false,
}: Props) {
  return (
    <div className={`relative flex items-center ${className}`}>
      {prefix && (
        <span
          className="absolute left-4 text-sm font-mono text-[var(--ink)] opacity-50 select-none pointer-events-none whitespace-nowrap"
          aria-hidden="true"
        >
          {prefix}
        </span>
      )}
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        autoFocus={autoFocus}
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        style={{
          paddingLeft: prefix ? `calc(${prefix.length}ch + 1.25rem)` : '1rem',
        }}
        className={[
          'w-full py-4 pr-4 rounded-xl',
          'bg-[var(--cream-2)] border border-[var(--coral-deep)]/20',
          'font-mono text-sm text-[var(--ink)]',
          'focus:outline-none focus:border-[var(--coral)] focus:ring-1 focus:ring-[var(--coral)]',
          'placeholder:text-[var(--ink)]/30',
          'disabled:opacity-50 disabled:cursor-not-allowed',
          'transition-colors',
        ].join(' ')}
      />
    </div>
  )
}
