import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../lib/constants'

export async function GET(_request: Request): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()

  const res = await fetch(`${API_BASE_URL}/matches`, {
    headers: { 'Authorization': `Bearer ${token}` },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
