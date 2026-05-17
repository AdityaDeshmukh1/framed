/**
 * POST /api/onboard — proxies to Go backend to kick off the scrape/enrich pipeline.
 * Body: { user_id: string, letterboxd_handle: string }
 * Returns: { job_id: string }
 */

import { API_BASE_URL } from '../../../lib/constants'

export async function POST(request: Request): Promise<Response> {
  const body = await request.json()

  const res = await fetch(`${API_BASE_URL}/onboard`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
