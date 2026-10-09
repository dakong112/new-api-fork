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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { usageLogSchema } from '../../data/schema'
import { UsageLogsProvider } from '../usage-logs-provider'
import { UsageLogsTable } from '../usage-logs-table'

const failure = (channel: number, status: number) => ({
  attempt: 1,
  channel_id: channel,
  status,
  elapsed_ms: 0,
  decision: {
    action: 'failure',
    reason: 'upstream_failure',
    source: 'upstream',
  },
})

async function renderLogs(retryReason: string, status: number) {
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
  const record = usageLogSchema.parse({
    id: 1,
    user_id: 1,
    created_at: 1788840000,
    type: 2,
    channel: 3,
    content: '',
    quota: 5000,
    other: JSON.stringify({
      admin_info: {
        use_channel: ['1', '3'],
        request_policy: [
          failure(1, status),
          {
            attempt: 1,
            elapsed_ms: 0,
            decision: { action: 'retry', reason: retryReason, source: 'group' },
          },
        ],
      },
    }),
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    let data: unknown = { quota: 0, rpm: 0, tpm: 0 }
    if (url.startsWith('/api/log?') || url.startsWith('/api/log/self?')) {
      data = { items: [record], total: 1 }
    } else if (
      url === '/api/group/' ||
      url === '/api/subscription/admin/plans'
    ) {
      data = []
    } else if (url === '/api/user/self/groups') {
      data = {}
    }
    return { data: { success: true, data } }
  })
  const root = createRootRoute()
  const auth = createRoute({ getParentRoute: () => root, id: '_authenticated' })
  const logs = createRoute({
    getParentRoute: () => auth,
    path: '/usage-logs/$section',
    component: () => <UsageLogsTable logCategory='common' />,
    validateSearch: (search: Record<string, unknown>) => search,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([logs])]),
    history: createMemoryHistory({ initialEntries: ['/usage-logs/common'] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <RouterProvider router={router} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  await screen.findAllByRole('button', { name: 'Retry Chain' })
  await waitFor(() => expect(client.isFetching()).toBe(0))
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  localStorage.clear()
})

test('marks a network retry and lists the status of each failed channel', async () => {
  const user = userEvent.setup()
  await renderLogs('network_error_retry', 524)

  expect(screen.getAllByText('Network retry')[0]).toBeVisible()
  await user.click(screen.getAllByRole('button', { name: 'Retry Chain' })[0])
  expect(await screen.findByText('#1 (524) → #3 ✓')).toBeVisible()
})

test('does not mark retries triggered by other statuses as network retries', async () => {
  await renderLogs('retry_status_matched', 429)

  expect(screen.queryByText('Network retry')).not.toBeInTheDocument()
})
