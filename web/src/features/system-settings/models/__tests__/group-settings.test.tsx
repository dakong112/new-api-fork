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
import { zodResolver } from '@hookform/resolvers/zod'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  within,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { describe, expect, it, vi } from 'vitest'
import { z } from 'zod'

import { api } from '@/lib/api'

import type { GroupUsage } from '../../api'
import { SettingsPageProvider } from '../../components/settings-page-context'
import { positiveIntegerSchema } from '../../utils/numeric-field'
import { GroupRatioForm } from '../group-ratio-form'

const defaults = {
  GroupRatio: '{"default":1,"vip":0.8}',
  TopupGroupRatio: '{"vip":1.2}',
  UserUsableGroups: '{"default":"Standard access","vip":"Premium access"}',
  GroupGroupRatio: '{}',
  AutoGroups: '["default","vip"]',
  GroupDisplayOrder: '[]',
  GroupFallbackGroups: '{}',
  GroupNetworkRetryGroups: '[]',
  MaxTokenAutoGroups: 5,
  DefaultUseAutoGroup: false,
  GroupSpecialUsableGroup: '{}',
}

const schema = z.object({
  GroupRatio: z.string(),
  TopupGroupRatio: z.string(),
  UserUsableGroups: z.string(),
  GroupGroupRatio: z.string(),
  AutoGroups: z.string(),
  GroupDisplayOrder: z.string(),
  GroupFallbackGroups: z.string(),
  GroupNetworkRetryGroups: z.string(),
  MaxTokenAutoGroups: positiveIntegerSchema('Enter a positive integer'),
  DefaultUseAutoGroup: z.boolean(),
  GroupSpecialUsableGroup: z.string(),
})

/** Query client with group usage pre-seeded, so the table never fetches it. */
function seededQueryClient(usage: GroupUsage[] = []) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  client.setQueryData(['group-usage'], usage)
  return client
}

function Fixture(props: {
  onSave?: (values: typeof defaults) => Promise<void>
  initial?: Partial<typeof defaults>
  isSaving?: boolean
  usage?: GroupUsage[]
}) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  const [queryClient] = useState(() => seededQueryClient(props.usage))
  const form = useForm({
    defaultValues: { ...defaults, ...props.initial },
    resolver: zodResolver(schema),
  })
  return (
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actions}>
        <div ref={setActions} />
        <GroupRatioForm
          form={form}
          onSave={props.onSave ?? (async () => {})}
          isSaving={props.isSaving ?? false}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

describe('group settings workspace', () => {
  it('filters by description and clears search without discarding edits', async () => {
    const user = userEvent.setup()
    render(<Fixture />)
    const search = screen.getByRole('textbox', {
      name: 'Search groups by name or description',
    })
    await user.type(search, 'PREMIUM')
    expect(screen.getAllByRole('textbox', { name: 'Group name' })).toHaveLength(
      1
    )
    expect(screen.getByRole('textbox', { name: 'Group name' })).toHaveValue(
      'vip'
    )
    await user.clear(screen.getByRole('spinbutton', { name: 'Ratio' }))
    await user.type(screen.getByRole('spinbutton', { name: 'Ratio' }), '0.6')
    await user.click(screen.getByRole('button', { name: 'Clear search' }))
    expect(screen.getAllByRole('textbox', { name: 'Group name' })).toHaveLength(
      2
    )
    expect(screen.getAllByRole('spinbutton', { name: 'Ratio' })[1]).toHaveValue(
      0.6
    )
    await user.type(search, 'missing')
    expect(screen.getByText('No results found')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Add group' }))
    expect(search).toHaveValue('')
    expect(screen.getByDisplayValue('group_1')).toBeVisible()
  })

  it('keeps unfinished visibility rules while switching sections and supports keyboard tabs', async () => {
    const user = userEvent.setup()
    render(
      <Fixture
        initial={{
          GroupSpecialUsableGroup: '{"vip":{"+:default":"Standard"}}',
        }}
      />
    )
    const pricing = screen.getByRole('tab', { name: 'Pricing groups' })
    pricing.focus()
    await user.keyboard('{ArrowRight}{Enter}')
    expect(
      screen.getByRole('tab', { name: 'Special ratio rules' })
    ).toHaveAttribute('aria-selected', 'true')
    expect(
      screen.queryByRole('table', { name: 'Pricing groups' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: 'Group visibility' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    expect(
      screen.getAllByRole('combobox', { name: 'Group name' })
    ).toHaveLength(2)
    await user.click(pricing)
    await user.click(screen.getByRole('tab', { name: 'Group visibility' }))
    expect(
      screen.getAllByRole('combobox', { name: 'Group name' })
    ).toHaveLength(2)
    const toggle = screen.getByRole('button', { name: 'Rules for vip' })
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
  })

  it('reorders and removes auto groups with named controls and saves across sections', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)
    await user.click(screen.getByRole('tab', { name: 'Auto group order' }))
    const list = screen.getByRole('list', { name: 'Auto group order' })
    expect(
      screen.getByRole('button', { name: 'Move default up' })
    ).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move vip down' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Move vip up' }))
    expect(within(list).getAllByRole('listitem')[0]).toHaveTextContent('vip')
    await user.click(
      screen.getByRole('switch', { name: 'Default to auto groups' })
    )
    await user.click(screen.getByRole('button', { name: 'Remove default' }))
    expect(screen.getByRole('button', { name: 'Move vip up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move vip down' })).toBeDisabled()
    await user.click(screen.getByRole('tab', { name: 'Pricing groups' }))
    await user.click(
      screen.getByRole('button', { name: 'Save group settings' })
    )
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        AutoGroups: '[\n  "vip"\n]',
        DefaultUseAutoGroup: true,
      })
    )
  })

  it('saves fallback groups in the order set with the move controls', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(
      <Fixture
        onSave={onSave}
        initial={{ GroupRatio: '{"default":1,"vip":0.8,"svip":0.5}' }}
      />
    )
    await user.click(screen.getByRole('tab', { name: 'Fallback groups' }))
    const picker = screen.getByRole('combobox', {
      name: 'Add fallback group for default',
    })
    await user.click(picker)
    await user.click(await screen.findByRole('option', { name: 'vip' }))
    await user.click(picker)
    await user.click(await screen.findByRole('option', { name: 'svip' }))

    const list = screen.getByRole('list', {
      name: 'Fallback groups for default',
    })
    expect(
      within(list)
        .getAllByRole('listitem')
        .map((item) => item.textContent)
    ).toEqual([expect.stringContaining('vip'), expect.stringContaining('svip')])
    await user.click(within(list).getByRole('button', { name: 'Move svip up' }))

    await user.click(
      screen.getByRole('button', { name: 'Save group settings' })
    )
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(
      JSON.parse(onSave.mock.calls[0]?.[0].GroupFallbackGroups ?? '')
    ).toEqual({ default: ['svip', 'vip'] })
  })

  it('saves the network retry switch of a group', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)
    await user.click(screen.getByRole('tab', { name: 'Fallback groups' }))
    const switches = screen.getAllByRole('switch', {
      name: 'Retry 502/504/524 on another channel',
    })
    expect(switches).toHaveLength(2)
    await user.click(switches[1])
    expect(switches[1]).toBeChecked()

    await user.click(
      screen.getByRole('button', { name: 'Save group settings' })
    )
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(
      JSON.parse(onSave.mock.calls[0]?.[0].GroupNetworkRetryGroups ?? '')
    ).toEqual(['vip'])
  })

  it('reorders from the drag handle with arrow keys, preserves unknown groups and saves the new order', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(
      <Fixture
        onSave={onSave}
        initial={{
          AutoGroups: '["default","vip","retired"]',
          MaxTokenAutoGroups: 1,
        }}
      />
    )
    await user.click(screen.getByRole('tab', { name: 'Auto group order' }))
    const list = screen.getByRole('list', { name: 'Auto group order' })
    const handle = screen.getByRole('button', {
      name: 'Drag default to reorder',
    })
    handle.focus()
    await user.keyboard('{ArrowUp}')
    expect(within(list).getAllByRole('listitem')[0]).toHaveTextContent(
      'default'
    )
    await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}')
    expect(within(list).getAllByRole('listitem')[2]).toHaveTextContent(
      'default'
    )
    expect(handle).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(within(list).getAllByRole('listitem')[1]).toHaveTextContent(
      'default'
    )
    expect(screen.getByText('Not in pricing table')).toBeVisible()
    await user.click(screen.getByRole('tab', { name: 'Pricing groups' }))
    await user.click(
      screen.getByRole('button', { name: 'Save group settings' })
    )
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(JSON.parse(onSave.mock.calls[0][0].AutoGroups)).toEqual([
      'vip',
      'default',
      'retired',
    ])
  })

  it('keeps a single auto group unchanged when using its drag handle keyboard controls', async () => {
    const user = userEvent.setup()
    render(<Fixture initial={{ AutoGroups: '["default"]' }} />)
    await user.click(screen.getByRole('tab', { name: 'Auto group order' }))
    const handle = screen.getByRole('button', {
      name: 'Drag default to reorder',
    })
    handle.focus()
    await user.keyboard('{ArrowUp}{ArrowDown}')
    const list = screen.getByRole('list', { name: 'Auto group order' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(1)
    expect(within(list).getByRole('listitem')).toHaveTextContent('default')
    expect(handle).toHaveFocus()
    expect(
      screen.getByRole('button', { name: 'Move default up' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Move default down' })
    ).toBeDisabled()
  })

  it('opens the auto section and focuses its invalid limit when saving from another section', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)
    await user.click(screen.getByRole('tab', { name: 'Auto group order' }))
    fireEvent.change(
      screen.getByRole('spinbutton', {
        name: 'Maximum custom groups per token',
      }),
      { target: { value: '0' } }
    )
    await user.click(screen.getByRole('tab', { name: 'Pricing groups' }))
    await user.click(
      screen.getByRole('button', { name: 'Save group settings' })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('tab', { name: 'Auto group order' })
      ).toHaveAttribute('aria-selected', 'true')
    )
    const input = screen.getByRole('spinbutton', {
      name: 'Maximum custom groups per token',
    })
    expect(input).toHaveAttribute('aria-invalid', 'true')
    await waitFor(() => expect(input).toHaveFocus())
    expect(screen.getByText('Enter a positive integer')).toBeVisible()
    expect(onSave).not.toHaveBeenCalled()
  })

  it('shows empty state guidance and disables save during a pending request', async () => {
    const user = userEvent.setup()
    render(
      <Fixture
        isSaving
        initial={{
          GroupRatio: '{}',
          TopupGroupRatio: '{}',
          UserUsableGroups: '{}',
          AutoGroups: '[]',
        }}
      />
    )
    expect(
      screen.getByText('No groups yet. Add a group to get started.')
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    await user.click(screen.getByRole('tab', { name: 'Special ratio rules' }))
    expect(
      screen.getByText('Base group ratios apply until you add an override.')
    ).toBeVisible()
    await user.click(screen.getByRole('tab', { name: 'Auto group order' }))
    expect(screen.getByText('No auto groups configured')).toBeVisible()
  })

  it('preserves pricing edits when switching between visual and JSON editors', async () => {
    const user = userEvent.setup()
    render(<Fixture />)
    await user.clear(
      screen.getAllByRole('textbox', { name: 'Group description' })[1]
    )
    await user.type(
      screen.getAllByRole('textbox', { name: 'Group description' })[1],
      'Updated description'
    )
    await user.click(screen.getByRole('button', { name: 'Switch to JSON' }))
    await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
    expect(
      screen.getAllByRole('textbox', { name: 'Group description' })[1]
    ).toHaveValue('Updated description')
  })

  it('orders pricing groups from the saved display order and saves a moved row', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(
      <Fixture
        onSave={onSave}
        initial={{ GroupDisplayOrder: '["vip","default"]' }}
      />
    )
    const names = () =>
      screen
        .getAllByRole('textbox', { name: 'Group name' })
        .map((input) => (input as HTMLInputElement).value)
    expect(names()).toEqual(['vip', 'default'])
    expect(screen.getByRole('button', { name: 'Move vip up' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Move default up' }))
    expect(names()).toEqual(['default', 'vip'])

    await user.click(
      screen.getByRole('button', { name: 'Save group settings' })
    )
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(JSON.parse(onSave.mock.calls[0][0].GroupDisplayOrder)).toEqual([
      'default',
      'vip',
    ])
  })

  it('flags groups without an enabled channel', () => {
    render(
      <Fixture
        usage={[
          {
            name: 'default',
            channels: 2,
            enabled_channels: 1,
            tokens: 0,
            users: 0,
          },
          {
            name: 'vip',
            channels: 1,
            enabled_channels: 0,
            tokens: 0,
            users: 0,
          },
        ]}
      />
    )

    expect(screen.getByText('1/2')).toBeVisible()
    expect(screen.getByText('None (0/1)')).toBeVisible()
  })

  it('warns about a group still in use that is missing from the table', () => {
    render(
      <Fixture
        usage={[
          {
            name: 'legacy-vip',
            channels: 3,
            enabled_channels: 2,
            tokens: 5,
            users: 12,
          },
        ]}
      />
    )

    expect(
      screen.getByText('Groups in use but missing from this table')
    ).toBeVisible()
    expect(
      screen.getByText('3 channels (2 enabled), 5 tokens, 12 users', {
        exact: false,
      })
    ).toBeVisible()
  })

  it('keeps existing group names read-only while new rows stay editable', async () => {
    const user = userEvent.setup()
    render(<Fixture />)
    const [existing] = screen.getAllByRole('textbox', { name: 'Group name' })
    expect(existing).toHaveAttribute('readonly')

    await user.click(screen.getByRole('button', { name: 'Add group' }))
    const added = screen.getByDisplayValue('group_1')
    expect(added).not.toHaveAttribute('readonly')
    expect(screen.queryByRole('button', { name: 'Rename group_1' })).toBeNull()
  })

  it('renames through the server and blocks renaming with unsaved edits', async () => {
    const user = userEvent.setup()
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: {
          channels: 3,
          users: 12,
          tokens: 5,
          subscription_plans: 0,
          user_subscriptions: 0,
          options: ['GroupRatio'],
        },
      },
    })
    render(<Fixture />)

    await user.click(screen.getByRole('button', { name: 'Rename vip' }))
    await user.type(
      screen.getByRole('textbox', { name: 'New group name' }),
      'premium'
    )
    await user.click(screen.getByRole('button', { name: 'Rename' }))

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/api/group/rename', {
        old_name: 'vip',
        new_name: 'premium',
      })
    )

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    // An unsaved edit disables rename, since it reloads settings afterwards.
    await user.clear(screen.getAllByRole('spinbutton', { name: 'Ratio' })[0])
    await user.type(
      screen.getAllByRole('spinbutton', { name: 'Ratio' })[0],
      '3'
    )
    expect(screen.getByRole('button', { name: 'Rename vip' })).toBeDisabled()
  })
})
