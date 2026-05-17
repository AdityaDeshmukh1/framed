/**
 * SoulPortrait — displays the user's generated soul portrait text and director note.
 * Server component.
 */

import { RuleDouble, DiamondDivider } from '../ui/Dividers'

/** Props for the SoulPortrait component */
interface Props {
  /** The main portrait prose (several sentences, serif italic) */
  portrait: string
  /** Optional director note — italicised attribution below the main text */
  directorNote?: string
  /** Whether to show decorative dividers */
  showDividers?: boolean
}

export function SoulPortrait({ portrait, directorNote, showDividers = true }: Props) {
  return (
    <div className="flex flex-col gap-4">
      {showDividers && <RuleDouble />}

      {/* Chapter kicker */}
      <p
        className="font-sans text-xs font-semibold uppercase tracking-[0.15em]"
        style={{ color: 'var(--coral)' }}
      >
        Soul Portrait
      </p>

      {/* Main portrait text */}
      <p
        className="font-serif italic text-[18px] leading-relaxed"
        style={{ color: 'var(--ink-deep)' }}
      >
        {portrait}
      </p>

      {directorNote && (
        <>
          {showDividers && <DiamondDivider />}
          {/* Director note */}
          <p
            className="font-serif italic text-sm leading-relaxed opacity-70"
            style={{ color: 'var(--ink)' }}
          >
            {directorNote}
          </p>
        </>
      )}

      {showDividers && <RuleDouble />}
    </div>
  )
}
