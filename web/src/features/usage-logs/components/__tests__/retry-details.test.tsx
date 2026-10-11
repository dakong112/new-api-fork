/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type { PolicyEvent } from '@/features/system-settings/request-policies/api'

import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'

const get = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api', () => ({ api: { get } }))

const attempt1: PolicyEvent[] = [
  {
    attempt: 1,
    channel_id: 45,
    elapsed_ms: 0,
    decision: {
      action: 'attempt',
      reason: 'channel_selected',
      source: 'routing',
    },
  },
  {
    attempt: 1,
    channel_id: 45,
    status: 524,
    elapsed_ms: 125000,
    decision: {
      action: 'failure',
      reason: 'upstream_error',
      source: 'routing',
    },
  },
]
const attempt2: PolicyEvent[] = [
  {
    attempt: 2,
    channel_id: 42,
    elapsed_ms: 125010,
    decision: {
      action: 'attempt',
      reason: 'channel_selected',
      source: 'routing',
    },
  },
  {
    attempt: 2,
    channel_id: 42,
    elapsed_ms: 139000,
    decision: { action: 'success', reason: 'upstream_ok', source: 'routing' },
  },
]

function makeLog(
  id: number,
  type: number,
  channel: number,
  events: PolicyEvent[]
): UsageLog {
  return {
    id,
    user_id: 1,
    created_at: 1,
    type,
    content: '',
    username: 'user',
    token_name: 'token',
    model_name: 'gpt-test',
    quota: type === 2 ? 716 : 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 139,
    is_stream: false,
    channel,
    channel_name: `channel-${channel}`,
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify({ admin_info: { request_policy: events } }),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

// Attempt 1 timed out on #45 (error log), attempt 2 succeeded on #42.
const errorLog = makeLog(1, 5, 45, attempt1)
const consumeLog = makeLog(2, 2, 42, [...attempt1, ...attempt2])

const queryClients: QueryClient[] = []

beforeEach(() => {
  get.mockResolvedValue({
    data: {
      success: true,
      data: { items: [errorLog, consumeLog], total: 2, page: 1, page_size: 50 },
    },
  })
})

function renderDetails(log: UsageLog, isAdmin: boolean): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['status'], {}, { updatedAt: Date.now() + 60_000 })
  queryClients.push(queryClient)
  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={log}
        isAdmin={isAdmin}
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
}

function logRequests(): string[] {
  return get.mock.calls
    .map((call) => String(call[0]))
    .filter((url) => url.startsWith('/api/log'))
}

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
  get.mockReset()
})

describe('usage log details retry process', () => {
  test('admin opening a retried request sees the retry process instead of the policy block', async () => {
    renderDetails(consumeLog, true)

    expect(
      await screen.findByRole('list', { name: 'Retry process' })
    ).toBeInTheDocument()
    expect(screen.queryByText('Request policy decisions')).toBeNull()
    expect(logRequests()[0]).toContain('request_id=req-1')
  })

  test('admin clicking view final record on a failed attempt switches to the final record', async () => {
    renderDetails(errorLog, true)
    expect(screen.getByText('(channel-45)')).toBeInTheDocument()

    await userEvent.click(
      await screen.findByRole('button', { name: 'View final record' })
    )

    expect(
      screen.queryByRole('button', { name: 'View final record' })
    ).toBeNull()
    expect(screen.getByRole('dialog', { name: /Consume/ })).toBeInTheDocument()
    expect(screen.getByText('(channel-42)')).toBeInTheDocument()
    expect(screen.queryByText('(channel-45)')).toBeNull()
  })

  test('admin expanding routing details on an intermediate error row sees the later attempt', async () => {
    renderDetails(errorLog, true)

    await userEvent.click(
      await screen.findByRole('button', { name: 'Routing decision details' })
    )

    const flow = screen.getByRole('list', { name: 'Request decision flow' })
    expect(within(flow).getAllByText(/Attempt 2/).length).toBeGreaterThan(0)
  })

  test('admin whose sibling fetch fails keeps the policy block without the retry process', async () => {
    get.mockRejectedValue(new Error('network down'))
    renderDetails(consumeLog, true)

    expect(
      await screen.findByText('Request policy decisions')
    ).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Retry process' })).toBeNull()
  })

  test('non-admin details do not request sibling logs or show the retry process', () => {
    renderDetails(consumeLog, false)

    expect(logRequests()).toEqual([])
    expect(screen.queryByRole('list', { name: 'Retry process' })).toBeNull()
  })

  test('admin opening a single-attempt request keeps the policy block', () => {
    renderDetails(makeLog(3, 2, 42, attempt2), true)

    expect(screen.getByText('Request policy decisions')).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Retry process' })).toBeNull()
    expect(logRequests()).toEqual([])
  })
})
