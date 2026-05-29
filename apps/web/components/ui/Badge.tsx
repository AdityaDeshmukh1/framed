/**
 * Badge component — score, tag, and count variants.
 * Server component — no interactivity needed.
 */

type Variant = 'score' | 'tag' | 'count'

/** Props for the Badge component */
interface Props {
  /** Visual style variant */
  variant?: Variant
  /** Badge content */
  children: React.ReactNode
  /** Additional CSS class names */
  className?: string
}

const variantStyles: Record<Variant, string> = {
  score: [
    'bg-[var(--teal)] text-white',
    'px-3 py-1 rounded-full',
    'font-mono text-sm font-semibold',
    'tabular-nums',
  ].join(' '),
  tag: [
    'bg-[var(--cream-2)] text-[var(--coral-deep)]',
    'px-3 py-1 rounded-full',
    'font-sans text-xs font-medium',
    'border border-[var(--coral-deep)]/20',
  ].join(' '),
  count: [
    'bg-[var(--coral)] text-white',
    'px-2 py-0.5 rounded-full',
    'font-mono text-xs font-semibold',
    'tabular-nums',
  ].join(' '),
}

export function Badge({ variant = 'tag', children, className = '' }: Props) {
  return (
    <span className={`inline-flex items-center ${variantStyles[variant]} ${className}`}>
      {children}
    </span>
  )
}
