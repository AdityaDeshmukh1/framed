'use client'

/**
 * Chat page — messaging interface with a pinned first-date film and live conversation.
 * Client component because it manages local message state and the input field.
 */

import { useState, useRef, useEffect } from 'react'
import { useParams } from 'next/navigation'
import { motion, AnimatePresence } from 'framer-motion'
import { PhoneShell } from '../../../components/layout/PhoneShell'
import { AppBar } from '../../../components/layout/AppBar'
import { ChatBubble } from '../../../components/features/ChatBubble'
import { FilmPoster } from '../../../components/features/FilmPoster'
import { RuleThin } from '../../../components/ui/Dividers'
import { useUser } from '@clerk/nextjs'

/** Animated typing indicator dots */
function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-4 py-2">
      {[0, 1, 2].map((i) => (
        <motion.span
          key={i}
          className="w-1.5 h-1.5 rounded-full"
          style={{ background: 'var(--ink)', opacity: 0.35 }}
          animate={{ opacity: [0.35, 0.7, 0.35], scale: [1, 1.2, 1] }}
          transition={{ duration: 1.2, delay: i * 0.2, repeat: Infinity, ease: 'easeInOut' }}
        />
      ))}
    </div>
  )
}

function SendIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M16 9L2 2l3 7-3 7 14-7Z" />
    </svg>
  )
}

export default function ChatPage() {
  const { id } = useParams<{ id: string }>()
  const { user } = useUser()
  const myId = user?.id ?? 'me'

  const match: { id: string; other_user: { display_name: string; letterboxd_handle: string }; first_date_film: { id: string; title: string; year: number } | null; score: number } = { id, other_user: { display_name: '—', letterboxd_handle: '' }, first_date_film: null, score: 0 }
  const { other_user, first_date_film, score } = match

  const [messages, setMessages] = useState<Array<{ id: string; match_id: string; sender_id: string; content: string; sent_at: string }>>([])
  const [inputValue, setInputValue] = useState('')
  const [isTyping, setIsTyping] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Scroll to bottom whenever messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, isTyping])

  function handleSend() {
    const text = inputValue.trim()
    if (!text) return

    // Append the user's message
    const newMsg = {
      id: `msg-${Date.now()}`,
      match_id: id,
      sender_id: myId,
      content: text,
      sent_at: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, newMsg])
    setInputValue('')

    // Simulate the other user typing then responding
    setIsTyping(true)
    setTimeout(() => {
      setIsTyping(false)
      setMessages((prev) => [
        ...prev,
        {
          id: `msg-reply-${Date.now()}`,
          match_id: id,
          sender_id: other_user.letterboxd_handle,
          content: '…',
          sent_at: new Date().toISOString(),
        },
      ])
    }, 2200)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <PhoneShell>
      {/* App bar with name + score */}
      <AppBar
        showBack
        backHref={`/match/${id}`}
        title={other_user.display_name}
        rightAction={
          <span
            className="font-mono text-xs font-semibold"
            style={{ color: 'var(--teal)' }}
          >
            {score}%
          </span>
        }
      />

      {/* ── Pinned first-date film ─── */}
      {first_date_film && (
        <div
          className="flex items-center gap-3 px-4 py-3"
          style={{ background: 'var(--coral-deep)' }}
        >
          <FilmPoster
            title={first_date_film.title}
            color="coral"
            small
            className="w-10 flex-shrink-0"
          />
          <div className="flex flex-col gap-0.5 flex-1 min-w-0">
            <p
              className="font-sans text-[10px] uppercase tracking-widest opacity-50"
              style={{ color: 'var(--cream)' }}
            >
              First date film
            </p>
            <p
              className="font-serif italic text-base truncate"
              style={{ color: 'var(--cream)' }}
            >
              {first_date_film.title}
            </p>
          </div>
          <button
            type="button"
            className="font-sans text-[10px] font-semibold uppercase tracking-widest flex-shrink-0 transition-opacity hover:opacity-80"
            style={{ color: 'var(--amber)' }}
          >
            Where to watch →
          </button>
        </div>
      )}

      <RuleThin />

      {/* ── Messages ─── */}
      <div
        className="flex-1 flex flex-col gap-3 px-4 py-4 overflow-y-auto"
        style={{ minHeight: 0, maxHeight: 'calc(100svh - 220px)' }}
      >
        <AnimatePresence initial={false}>
          {messages.map((msg) => (
            <motion.div
              key={msg.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2, ease: 'easeOut' }}
            >
              <ChatBubble
                content={msg.content}
                isMine={msg.sender_id === myId}
                senderName={other_user.display_name}
                sentAt={msg.sent_at}
              />
            </motion.div>
          ))}
        </AnimatePresence>

        {/* Typing indicator */}
        <AnimatePresence>
          {isTyping && (
            <motion.div
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 4 }}
              className="self-start"
            >
              <div
                className="px-4 py-2 rounded-2xl rounded-bl-md"
                style={{ background: 'var(--cream-2)' }}
              >
                <TypingIndicator />
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Scroll anchor */}
        <div ref={messagesEndRef} />
      </div>

      {/* ── Input bar ─── */}
      <div
        className="flex items-center gap-3 px-4 py-3 sticky bottom-0"
        style={{
          background: 'var(--cream)',
          borderTop: '1px solid rgba(74,27,12,0.1)',
        }}
      >
        <input
          type="text"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Say something…"
          className="flex-1 px-4 py-3 rounded-full font-sans text-sm outline-none transition-all"
          style={{
            background: 'var(--cream-2)',
            color: 'var(--ink)',
            border: '1.5px solid transparent',
          }}
          onFocus={(e) => { e.target.style.borderColor = 'var(--coral)' }}
          onBlur={(e) => { e.target.style.borderColor = 'transparent' }}
        />
        <button
          type="button"
          onClick={handleSend}
          disabled={!inputValue.trim()}
          className="w-11 h-11 rounded-full flex items-center justify-center flex-shrink-0 transition-all active:scale-95"
          style={{
            background: inputValue.trim() ? 'var(--coral)' : 'var(--cream-2)',
            color: inputValue.trim() ? 'white' : 'var(--ink)',
            opacity: inputValue.trim() ? 1 : 0.5,
          }}
          aria-label="Send message"
        >
          <SendIcon />
        </button>
      </div>
    </PhoneShell>
  )
}
