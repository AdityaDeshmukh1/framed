/**
 * ChatBubble — a single chat message bubble, aligned left (theirs) or right (yours).
 * Server component — purely presentational.
 */

import { MOCK_USER_ID } from '../../lib/constants'

/** Props for the ChatBubble component */
interface Props {
  /** The message text */
  content: string
  /** The ID of the sender — compared against MOCK_USER_ID to determine alignment */
  senderId: string
  /** Optional display name for the sender (shown on left bubbles) */
  senderName?: string
  /** ISO timestamp */
  sentAt?: string
}

function formatBubbleTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true })
}

export function ChatBubble({ content, senderId, senderName, sentAt }: Props) {
  const isMine = senderId === MOCK_USER_ID

  return (
    <div
      className={`flex flex-col gap-1 max-w-[80%] ${isMine ? 'self-end items-end' : 'self-start items-start'}`}
    >
      {!isMine && senderName && (
        <span
          className="font-sans text-[10px] font-semibold uppercase tracking-widest px-1"
          style={{ color: 'var(--ink)', opacity: 0.4 }}
        >
          {senderName}
        </span>
      )}

      <div
        className="px-4 py-3 rounded-2xl"
        style={
          isMine
            ? {
                background: 'var(--coral)',
                color: 'white',
                borderBottomRightRadius: '6px',
              }
            : {
                background: 'var(--cream-2)',
                color: 'var(--ink-deep)',
                borderBottomLeftRadius: '6px',
              }
        }
      >
        <p className="font-sans text-sm leading-relaxed">{content}</p>
      </div>

      {sentAt && (
        <span
          className="font-mono text-[10px] px-1"
          style={{ color: 'var(--ink)', opacity: 0.35 }}
        >
          {formatBubbleTime(sentAt)}
        </span>
      )}
    </div>
  )
}
