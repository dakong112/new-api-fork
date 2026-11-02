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
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

const getNotice = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api', () => ({ getNotice }))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {}, loading: false }),
}))

const { NOTICE_POLL_INTERVAL_MS, useNotifications } =
  await import('../use-notifications')

function renderNotifications() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = (props: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
  return renderHook(() => useNotifications(), { wrapper })
}

describe('system notice polling', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getNotice.mockResolvedValue({ success: true, data: 'maintenance' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  test('checks for a new notice every five minutes, not every minute', async () => {
    renderNotifications()
    await act(() => vi.advanceTimersByTimeAsync(0))
    expect(getNotice).toHaveBeenCalledTimes(1)

    await act(() => vi.advanceTimersByTimeAsync(NOTICE_POLL_INTERVAL_MS - 1000))
    expect(getNotice).toHaveBeenCalledTimes(1)

    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(getNotice).toHaveBeenCalledTimes(2)
  })

  test('returning to the tab within five minutes does not check again', async () => {
    renderNotifications()
    await act(() => vi.advanceTimersByTimeAsync(0))
    expect(getNotice).toHaveBeenCalledTimes(1)

    await act(() => vi.advanceTimersByTimeAsync(2 * 60 * 1000))
    await act(async () => {
      window.dispatchEvent(new Event('visibilitychange'))
      window.dispatchEvent(new Event('focus'))
    })

    expect(getNotice).toHaveBeenCalledTimes(1)
  })
})
