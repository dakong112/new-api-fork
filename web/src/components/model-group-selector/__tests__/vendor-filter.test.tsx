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
import { describe, expect, it, vi } from 'vitest'

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
  selectedGroup = '',
  onGroupChange = vi.fn()
) {
  const user = userEvent.setup()
  render(
    <ModelGroupSelector
      selectedModel='m'
      models={[{ value: 'm', label: 'm' }]}
      onModelChange={vi.fn()}
      selectedGroup={selectedGroup}
      groups={groups.map((value) => ({ value, label: value }))}
      onGroupChange={onGroupChange}
      groupVendors={groupVendors}
    />
  )
  await user.click(screen.getByRole('combobox'))
  return user
}

describe('ModelGroupSelector vendor filter', () => {
  it("when a vendor is clicked, only that vendor's groups remain", async () => {
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

  it('when pricing is ready, a group without enabled models is disabled and labelled', async () => {
    await openSelector({ ...index, ready: true }, ['ds', 'empty'])
    expect(screen.getByRole('button', { name: /^empty/ })).toBeDisabled()
    expect(screen.getByText('Temporarily unavailable')).toBeVisible()
    expect(screen.getByRole('button', { name: /^ds/ })).toBeEnabled()
  })

  it('when pricing is not ready, no group is disabled', async () => {
    await openSelector({ ...index, ready: false }, ['empty'])
    expect(screen.getByRole('button', { name: /^empty/ })).toBeEnabled()
  })

  it('when a vendor is clicked, the selected group and auto stay visible', async () => {
    const user = await openSelector(index, ['ds', 'zp', 'auto'], 'zp')
    await user.click(
      within(screen.getByRole('group', { name: 'Vendor' })).getByRole(
        'button',
        { name: 'DeepSeek' }
      )
    )
    expect(screen.getByRole('button', { name: /^ds/ })).toBeVisible()
    expect(screen.getByRole('button', { name: /^zp/ })).toBeVisible()
    expect(screen.getByRole('button', { name: /^auto/ })).toBeVisible()
  })

  it('when a vendor is clicked and the selected group is not in it, switches to its first available group', async () => {
    const onGroupChange = vi.fn()
    const user = await openSelector(
      { ...index, ready: true },
      ['zp', 'ds'],
      'zp',
      onGroupChange
    )
    await user.click(
      within(screen.getByRole('group', { name: 'Vendor' })).getByRole(
        'button',
        { name: 'DeepSeek' }
      )
    )
    expect(onGroupChange).toHaveBeenCalledWith('ds')
  })

  it('when a vendor is clicked and the selected group is in it, keeps the group', async () => {
    const onGroupChange = vi.fn()
    const user = await openSelector(index, ['ds', 'zp'], 'ds', onGroupChange)
    await user.click(
      within(screen.getByRole('group', { name: 'Vendor' })).getByRole(
        'button',
        { name: 'DeepSeek' }
      )
    )
    expect(onGroupChange).not.toHaveBeenCalled()
  })
})
