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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { buildGroupVendorIndex } from '@/lib/group-vendors'

import { ModelGroupSelector } from '../../model-group-selector'

const index = buildGroupVendorIndex(
  [
    { vendor_id: 1, enable_groups: ['ds'] },
    { vendor_id: 2, enable_groups: ['zp'] },
  ],
  [
    { id: 1, name: 'DeepSeek' },
    { id: 2, name: '智谱' },
  ]
)

async function openSelector(
  groupVendors: Parameters<typeof ModelGroupSelector>[0]['groupVendors'],
  groups: string[],
  selectedGroup = ''
) {
  const user = userEvent.setup()
  render(
    <ModelGroupSelector
      selectedModel='m'
      models={[{ value: 'm', label: 'm' }]}
      onModelChange={vi.fn()}
      selectedGroup={selectedGroup}
      groups={groups.map((value) => ({ value, label: value }))}
      onGroupChange={vi.fn()}
      groupVendors={groupVendors}
    />
  )
  await user.click(screen.getByRole('combobox'))
  return user
}

describe('ModelGroupSelector vendor filter', () => {
  beforeEach(() => {
    // useIsMobile reads innerWidth; pin the desktop popover layout.
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      value: 1024,
    })
  })

  it('shows only the chosen vendor groups', async () => {
    const user = await openSelector(index, ['ds', 'zp'])
    await user.click(
      within(screen.getByRole('group', { name: 'Vendor' })).getByRole(
        'button',
        { name: 'DeepSeek' }
      )
    )
    expect(screen.getByRole('button', { name: /^ds/ })).toBeVisible()
    expect(screen.queryByRole('button', { name: /^zp/ })).toBeNull()
  })

  it('greys out a group without enabled models', async () => {
    await openSelector({ ...index, ready: true }, ['ds', 'empty'])
    expect(screen.getByRole('button', { name: /^empty/ })).toBeDisabled()
    expect(screen.getByText('Temporarily unavailable')).toBeVisible()
    expect(screen.getByRole('button', { name: /^ds/ })).toBeEnabled()
  })

  it('does not grey out groups before pricing is ready', async () => {
    await openSelector({ ...index, ready: false }, ['empty'])
    expect(screen.getByRole('button', { name: /^empty/ })).toBeEnabled()
  })
})
