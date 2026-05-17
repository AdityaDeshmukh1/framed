/**
 * POST /api/match — proxies to Go backend to fetch match candidates.
 * Body: { user_id: string }
 * Returns: Match[]
 */

import { API_BASE_URL } from '../../../lib/constants'

export async function POST(request: Request): Promise<Response> {
  const body = await request.json()

  const res = await fetch(`${API_BASE_URL}/match`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
