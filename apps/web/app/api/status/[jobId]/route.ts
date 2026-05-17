/**
 * GET /api/status/[jobId] — polls the Go backend for scrape job status.
 * Returns: ScrapeJob object
 */

import { API_BASE_URL } from '../../../../lib/constants'

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ jobId: string }> }
): Promise<Response> {
  const { jobId } = await params

  const res = await fetch(`${API_BASE_URL}/status/${jobId}`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
