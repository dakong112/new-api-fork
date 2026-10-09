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
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { MonitoringSettingsSection } from '../monitoring-settings-section'

function Fixture(props: { threshold: string }) {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <MonitoringSettingsSection
          defaultValues={{
            QuotaRemindThreshold: '1000',
            ChannelBalanceAlertThreshold: props.threshold,
            'perf_metrics_setting.enabled': true,
            'perf_metrics_setting.flush_interval': 5,
            'perf_metrics_setting.bucket_time': 'hour',
            'perf_metrics_setting.retention_days': 0,
          }}
        />
      </SettingsPageProvider>
    </>
  )
}

async function renderSettings(threshold = '0') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => <Fixture threshold={threshold} />,
    }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return screen.findByRole('spinbutton', {
    name: 'Channel balance alert (USD)',
  })
}

beforeEach(() => {
  vi.restoreAllMocks()
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

test('saving a channel balance alert threshold sends only that option', async () => {
  const user = userEvent.setup()
  const input = await renderSettings('0')
  await user.clear(input)
  await user.type(input, '5')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/option/', {
      key: 'ChannelBalanceAlertThreshold',
      value: '5',
    })
  )
  expect(api.put).toHaveBeenCalledTimes(1)
})

test('clearing the threshold turns the alert off by saving zero', async () => {
  const user = userEvent.setup()
  const input = await renderSettings('5')
  expect(input).toHaveValue(5)
  await user.clear(input)
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/option/', {
      key: 'ChannelBalanceAlertThreshold',
      value: '0',
    })
  )
})

test('a negative threshold shows a field error and is not saved', async () => {
  const input = await renderSettings('0')
  fireEvent.change(input, { target: { value: '-1' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() => expect(input).toHaveAttribute('aria-invalid', 'true'))
  expect(api.put).not.toHaveBeenCalled()
})
