import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../../lib/constants'

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ jobId: string }> }
): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()
  const { jobId } = await params

  const res = await fetch(`${API_BASE_URL}/status/${jobId}`, {
    headers: { 'Authorization': `Bearer ${token}` },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
