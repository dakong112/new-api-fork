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
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { VendorRulesCard } from '../vendor-rules-card'

const rules = JSON.stringify([
  { name: 'OpenAI', icon: 'OpenAI', keywords: ['gpt', 'o1'] },
  { name: 'Claude', keywords: ['claude'] },
])

function Fixture(props: { value: string }) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { retry: false },
          mutations: { retry: false },
        },
      })
  )
  return (
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actions}>
        <div ref={setActions} />
        <VendorRulesCard defaultValue={props.value} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

function mockSave() {
  return vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true, message: '' } })
}

function savedValue(put: ReturnType<typeof mockSave>) {
  expect(put).toHaveBeenCalledTimes(1)
  const [url, body] = put.mock.calls[0]
  expect(url).toBe('/api/option/')
  return body as { key: string; value: string }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('vendor rules settings', () => {
  it('saves edited names and comma-separated keywords as trimmed JSON', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    const name = screen.getAllByRole('textbox', { name: 'Name' })[1]
    await user.clear(name)
    await user.type(name, 'Anthropic')
    const keywords = screen.getAllByRole('textbox', { name: 'Keywords' })[1]
    await user.clear(keywords)
    await user.type(keywords, ' claude ,, Sonnet , ')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    const body = savedValue(put)
    expect(body.key).toBe('VendorRules')
    expect(JSON.parse(body.value)).toEqual([
      { name: 'OpenAI', icon: 'OpenAI', keywords: ['gpt', 'o1'] },
      { name: 'Anthropic', keywords: ['claude', 'Sonnet'] },
    ])
  })

  it('blocks saving duplicate names ignoring case and marks the row', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    const name = screen.getAllByRole('textbox', { name: 'Name' })[1]
    await user.clear(name)
    await user.type(name, 'openai')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(put).not.toHaveBeenCalled()
    expect(name).toHaveAttribute('aria-invalid', 'true')
    expect(
      screen.getAllByText('Vendor name must be unique').length
    ).toBeGreaterThan(0)
  })

  it('blocks saving a new row without name or keywords', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    await user.click(screen.getByRole('button', { name: 'Add vendor' }))
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(put).not.toHaveBeenCalled()
    expect(screen.getByText('Vendor name is required')).toBeInTheDocument()
    expect(
      screen.getByText('At least one keyword is required')
    ).toBeInTheDocument()
  })

  it('submits an empty value after confirming restore defaults', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    await user.click(screen.getByRole('button', { name: 'Restore defaults' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(
      within(dialog).getByRole('button', { name: 'Restore defaults' })
    )

    expect(savedValue(put)).toEqual({ key: 'VendorRules', value: '' })
  })

  it('saves the new order after moving a vendor up', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    await user.click(screen.getByRole('button', { name: 'Move Claude up' }))
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      JSON.parse(savedValue(put).value).map((r: { name: string }) => r.name)
    ).toEqual(['Claude', 'OpenAI'])
  })

  it('saves the new order after dragging a vendor above another', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    const dataTransfer = {
      setData: () => {},
      setDragImage: () => {},
      effectAllowed: '',
      dropEffect: '',
    }
    fireEvent.dragStart(
      screen.getByRole('button', { name: 'Drag Claude to reorder' }),
      { dataTransfer }
    )
    const openaiRow = screen
      .getAllByRole('textbox', { name: 'Name' })[0]
      .closest('tr') as HTMLElement
    fireEvent.dragOver(openaiRow, { dataTransfer, clientY: -1 })
    fireEvent.drop(openaiRow, { dataTransfer, clientY: -1 })
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      JSON.parse(savedValue(put).value).map((r: { name: string }) => r.name)
    ).toEqual(['Claude', 'OpenAI'])
  })
})
