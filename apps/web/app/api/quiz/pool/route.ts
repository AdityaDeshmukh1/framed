import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../../lib/constants'

/** Quiz cards with embeddings (~1MB): streamed through rather than re-parsed. */
export async function GET(): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()
  const res = await fetch(`${API_BASE_URL}/quiz/pool`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  return new Response(res.body, {
    status: res.status,
    headers: { 'Content-Type': 'application/json', 'Cache-Control': 'private, max-age=600' },
  })
}
