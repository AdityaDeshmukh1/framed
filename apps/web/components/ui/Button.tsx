'use client'

/**
 * Button component — primary, ghost, and icon variants.
 * Uses Framer Motion for press feedback.
 */

import { motion } from 'framer-motion'

type Variant = 'primary' | 'ghost' | 'icon'

/** Props for the Button component */
interface Props {
  /** Visual style variant */
  variant?: Variant
  /** Button label or icon content */
  children: React.ReactNode
  /** Click handler */
  onClick?: () => void
  /** Disabled state */
  disabled?: boolean
  /** HTML button type */
  type?: 'button' | 'submit' | 'reset'
  /** Additional CSS class names */
  className?: string
  /** Full width (default true for primary/ghost) */
  fullWidth?: boolean
}

const variantStyles: Record<Variant, string> = {
  primary: [
    'bg-[var(--coral)] text-white',
    'w-full py-4 px-6 rounded-full',
    'font-sans font-semibold text-base tracking-wide',
    'transition-opacity hover:opacity-90',
    'disabled:opacity-40 disabled:cursor-not-allowed',
  ].join(' '),
  ghost: [
    'bg-transparent text-[var(--coral)] border border-[var(--coral)]',
    'w-full py-4 px-6 rounded-full',
    'font-sans font-semibold text-base tracking-wide',
    'transition-opacity hover:opacity-80',
    'disabled:opacity-40 disabled:cursor-not-allowed',
  ].join(' '),
  icon: [
    'bg-transparent text-[var(--ink)] p-2 rounded-full',
    'flex items-center justify-center',
    'hover:bg-black/5 transition-colors',
    'disabled:opacity-40 disabled:cursor-not-allowed',
  ].join(' '),
}

export function Button({
  variant = 'primary',
  children,
  onClick,
  disabled = false,
  type = 'button',
  className = '',
}: Props) {
  return (
    <motion.button
      type={type}
      onClick={onClick}
      disabled={disabled}
      whileTap={{ scale: disabled ? 1 : 0.97 }}
      className={`${variantStyles[variant]} ${className}`}
    >
      {children}
    </motion.button>
  )
}
