import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../lib/constants'

export async function GET(_request: Request): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()

  const res = await fetch(`${API_BASE_URL}/soul`, {
    headers: { 'Authorization': `Bearer ${token}` },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}

export async function POST(_request: Request): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()

  const res = await fetch(`${API_BASE_URL}/soul`, {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${token}` },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
