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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useNotificationStore } from '@/stores/notification-store'

import { NoticeDialog } from '../notice-dialog'

function mockNotice(notice: string) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: { success: true, data: url === '/api/notice' ? notice : {} },
  }))
}

function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <NoticeDialog />
    </QueryClientProvider>
  )
}

function footerButton(name: string) {
  // The dialog's corner X also exposes "Close"; the footer button has text.
  return screen
    .getAllByRole('button', { name })
    .find((button) => button.textContent === name) as HTMLElement
}

describe('NoticeDialog', () => {
  beforeEach(() => {
    localStorage.clear()
    useNotificationStore.setState({
      lastReadNotice: '',
      readAnnouncementKeys: [],
      closedUntilDate: null,
      closedNotice: '',
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('opens with the notice when it has not been read', async () => {
    mockNotice('Maintenance on Friday')
    renderDialog()

    expect(await screen.findByRole('dialog')).toBeTruthy()
    expect(screen.getByText('Maintenance on Friday')).toBeTruthy()
  })

  it('stays closed after Close marks the notice as read', async () => {
    mockNotice('Maintenance on Friday')
    const view = renderDialog()
    await screen.findByRole('dialog')

    await userEvent.click(footerButton('Close'))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(useNotificationStore.getState().lastReadNotice).toBe(
      'Maintenance on Friday'
    )
    view.unmount()
    renderDialog()
    await waitFor(() => expect(api.get).toHaveBeenCalled())
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('hides for today without marking read when Close Today is clicked', async () => {
    mockNotice('Maintenance on Friday')
    renderDialog()
    await screen.findByRole('dialog')

    await userEvent.click(footerButton('Close Today'))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(useNotificationStore.getState().closedUntilDate).toBe(
      new Date().toDateString()
    )
    expect(useNotificationStore.getState().lastReadNotice).toBe('')
  })

  it('opens when a notice is published after the page loaded', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    mockNotice('')
    renderDialog()
    await waitFor(() => expect(api.get).toHaveBeenCalled())
    expect(screen.queryByRole('dialog')).toBeNull()

    mockNotice('New notice')
    await vi.advanceTimersByTimeAsync(60 * 1000)

    expect(await screen.findByRole('dialog')).toBeTruthy()
    expect(screen.getByText('New notice')).toBeTruthy()
  })

  it('opens a newly published notice even after Close Today on the previous one', async () => {
    mockNotice('Maintenance on Friday')
    const view = renderDialog()
    await screen.findByRole('dialog')
    await userEvent.click(footerButton('Close Today'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    view.unmount()
    mockNotice('Maintenance moved to Saturday')
    renderDialog()

    expect(await screen.findByRole('dialog')).toBeTruthy()
    expect(screen.getByText('Maintenance moved to Saturday')).toBeTruthy()
  })

  it('does not open when the notice is empty', async () => {
    mockNotice('')
    renderDialog()

    await waitFor(() => expect(api.get).toHaveBeenCalled())
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
