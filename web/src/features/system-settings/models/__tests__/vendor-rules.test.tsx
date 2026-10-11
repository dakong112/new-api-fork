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
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
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

    const name = screen.getByRole('textbox', { name: 'Vendor name, row 2' })
    await user.clear(name)
    await user.type(name, 'Anthropic')
    const keywords = screen.getByRole('textbox', { name: 'Keywords, row 2' })
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

    const name = screen.getByRole('textbox', { name: 'Vendor name, row 2' })
    await user.clear(name)
    await user.type(name, 'openai')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(put).not.toHaveBeenCalled()
    expect(name).toHaveAttribute('aria-invalid', 'true')
    expect(
      within(name.closest('td') as HTMLElement).getByText(
        'Vendor name must be unique'
      )
    ).toBeInTheDocument()
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
      .getByRole('textbox', { name: 'Vendor name, row 1' })
      .closest('tr') as HTMLElement
    fireEvent.dragOver(openaiRow, { dataTransfer, clientY: -1 })
    fireEvent.drop(openaiRow, { dataTransfer, clientY: -1 })
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      JSON.parse(savedValue(put).value).map((r: { name: string }) => r.name)
    ).toEqual(['Claude', 'OpenAI'])
  })

  it('saves the new order after pressing ArrowUp on a drag handle', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    screen.getByRole('button', { name: 'Drag Claude to reorder' }).focus()
    await user.keyboard('{ArrowUp}')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      JSON.parse(savedValue(put).value).map((r: { name: string }) => r.name)
    ).toEqual(['Claude', 'OpenAI'])
  })

  it('saves the new order after pressing ArrowDown on a drag handle', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    screen.getByRole('button', { name: 'Drag OpenAI to reorder' }).focus()
    await user.keyboard('{ArrowDown}')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      JSON.parse(savedValue(put).value).map((r: { name: string }) => r.name)
    ).toEqual(['Claude', 'OpenAI'])
  })

  it('rebuilds the rows when the stored value changes', () => {
    const { rerender } = render(<Fixture value={rules} />)

    rerender(
      <Fixture
        value={JSON.stringify([{ name: 'Gemini', keywords: ['gemini'] }])}
      />
    )

    expect(
      screen.getByRole('textbox', { name: 'Vendor name, row 1' })
    ).toHaveValue('Gemini')
    expect(
      screen.queryByRole('textbox', { name: 'Vendor name, row 2' })
    ).not.toBeInTheDocument()
  })

  it('discards edits after restoring defaults when the stored value is unchanged', async () => {
    const user = userEvent.setup()
    mockSave()
    render(<Fixture value={rules} />)

    const name = screen.getByRole('textbox', { name: 'Vendor name, row 1' })
    await user.clear(name)
    await user.type(name, 'Edited')
    await user.click(screen.getByRole('button', { name: 'Restore defaults' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Restore defaults',
      })
    )

    await waitFor(() =>
      expect(
        screen.getByRole('textbox', { name: 'Vendor name, row 1' })
      ).toHaveValue('OpenAI')
    )
  })

  it('disables saving and explains when the stored value cannot be read', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value='not json' />)

    expect(screen.getByRole('alert')).toHaveTextContent(
      'The stored vendor rules could not be read. Restore defaults to continue.'
    )
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Restore defaults' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Restore defaults',
      })
    )
    expect(savedValue(put)).toEqual({ key: 'VendorRules', value: '' })
  })

  it('asks for confirmation before saving an empty rule list', async () => {
    const user = userEvent.setup()
    const put = mockSave()
    render(<Fixture value={rules} />)

    for (const button of screen.getAllByRole('button', { name: 'Delete' })) {
      await user.click(button)
    }
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(put).not.toHaveBeenCalled()

    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent(
      'Save an empty rule list? Every model will be shown under Other.'
    )
    await user.click(
      within(dialog).getByRole('button', { name: 'Save Changes' })
    )

    expect(savedValue(put)).toEqual({ key: 'VendorRules', value: '[]' })
  })

  it('labels the drag handle with the row number when the name is empty', async () => {
    const user = userEvent.setup()
    render(<Fixture value={rules} />)

    await user.click(screen.getByRole('button', { name: 'Add vendor' }))

    expect(
      screen.getByRole('button', { name: 'Drag Row 3 to reorder' })
    ).toBeInTheDocument()
  })
})
