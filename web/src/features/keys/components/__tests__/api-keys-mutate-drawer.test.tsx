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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { ApiKeysProvider } = await import('../api-keys-provider')
const { ApiKeysMutateDrawer } = await import('../api-keys-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
  put: ApiMethod
}
type RenderedDrawer = {
  queryClient: InstanceType<typeof QueryClient>
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post
const originalPut = apiClient.put
let renderedDrawer: RenderedDrawer | null = null

function installApiFixtures(createdPayloads: Array<Record<string, unknown>>) {
  apiClient.get = async (url) => {
    switch (url) {
      case '/api/status':
        return { data: { data: { default_use_auto_group: true } } }
      case '/api/user/models':
        return { data: { success: true, data: [] } }
      case '/api/user/self/groups':
        return {
          data: {
            success: true,
            data: {
              auto: { desc: 'Automatic routing', ratio: 'auto' },
              default: {
                desc: 'Standard access',
                ratio: 1,
                fallback_groups: ['vip'],
              },
              vip: { desc: 'Priority access', ratio: 2 },
              empty: { desc: 'No channels', ratio: 1 },
              relay: {
                desc: 'Fallback only',
                ratio: 1,
                fallback_groups: ['vip'],
              },
            },
          },
        }
      case '/api/pricing':
        return {
          data: {
            success: true,
            data: [
              {
                model_name: 'deepseek-v4',
                vendor_id: 1,
                enable_groups: ['vip'],
              },
              { model_name: 'glm-5', vendor_id: 2, enable_groups: ['default'] },
            ],
            vendors: [
              { id: 1, name: 'DeepSeek' },
              { id: 2, name: '智谱' },
            ],
            group_ratio: {},
            usable_group: {},
            supported_endpoint: {},
            auto_groups: [],
          },
        }
      case '/api/token/auto-groups':
        return {
          data: {
            success: true,
            data: { groups: ['vip', 'default'], max_count: 3 },
          },
        }
      case `/api/token/${emptyGroupKey.id}`:
        return { data: { success: true, data: emptyGroupKey } }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  }
  apiClient.post = async (url, data) => {
    expect(url).toBe('/api/token/')
    expect(data && typeof data === 'object').toBeTruthy()
    createdPayloads.push(data as Record<string, unknown>)
    return { data: { success: true, data: {} } }
  }
}

const emptyGroupKey = {
  id: 7,
  name: 'legacy',
  key: 'sk-legacy',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: true,
  expired_time: -1,
  created_time: 0,
  accessed_time: 0,
  group: 'empty',
  auto_groups: null,
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}

async function renderDrawer(currentRow?: typeof emptyGroupKey): Promise<void> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(
    ['status'],
    { default_use_auto_group: true },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-models'],
    { success: true, data: [] },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-groups'],
    {
      success: true,
      data: {
        auto: { desc: 'Automatic routing', ratio: 'auto' },
        default: {
          desc: 'Standard access',
          ratio: 1,
          fallback_groups: ['vip'],
        },
        vip: { desc: 'Priority access', ratio: 2 },
        empty: { desc: 'No channels', ratio: 1 },
        relay: { desc: 'Fallback only', ratio: 1, fallback_groups: ['vip'] },
      },
    },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['token-auto-groups'],
    {
      success: true,
      data: { groups: ['vip', 'default'], max_count: 3 },
    },
    { updatedAt: freshAt }
  )
  renderedDrawer = { queryClient }

  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ApiKeysProvider>
          <ApiKeysMutateDrawer
            open
            onOpenChange={() => undefined}
            currentRow={currentRow}
          />
        </ApiKeysProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
  await waitFor(
    () => {
      const saveButton = findButton('Save changes', false)
      expect(saveButton).toBeEnabled()
    },
    { timeout: 1500 }
  )
}

function findButton(text: string, required: true): HTMLButtonElement
function findButton(text: string, required: false): HTMLButtonElement | null
function findButton(text: string, required = true): HTMLButtonElement | null {
  const button = screen
    .queryAllByRole<HTMLButtonElement>('button')
    .find((candidate) => candidate.textContent?.includes(text))
  if (required && !button) {
    throw new Error(`Expected button containing "${text}"`)
  }
  return button ?? null
}

function getControlByLabel(labelText: 'Name' | 'Quantity'): HTMLInputElement
function getControlByLabel(labelText: 'Group'): HTMLButtonElement
function getControlByLabel(labelText: 'Auto group order'): HTMLElement
function getControlByLabel(labelText: string): HTMLElement {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  if (!label) {
    throw new Error(`Expected label "${labelText}"`)
  }

  const control =
    label.control ??
    label
      .closest('[data-slot="form-item"]')
      ?.querySelector<HTMLElement>(
        '[data-slot="form-control"], input, textarea, button[role="combobox"], [role="group"]'
      )
  if (!control) {
    throw new Error(`Expected control for label "${labelText}"`)
  }
  return control
}

function changeInput(input: HTMLInputElement, value: string): void {
  fireEvent.input(input, { target: { value } })
}

function selectComboboxOption(
  trigger: HTMLButtonElement,
  optionDescription: string
): void {
  fireEvent.click(trigger)
  const option = [
    ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
  ].find((candidate) => candidate.textContent?.includes(optionDescription))
  if (!option) {
    throw new Error(`Expected option containing "${optionDescription}"`)
  }
  fireEvent.click(option)
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  apiClient.put = originalPut
  localStorage.clear()
  if (renderedDrawer) {
    renderedDrawer.queryClient.clear()
    renderedDrawer = null
  }
})

describe('API keys mutate drawer Auto group integration', () => {
  test('inherits the root Auto order and sends an empty override for every batch-created key', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderDrawer()

    const groupTrigger = getControlByLabel('Group')
    expect(groupTrigger.textContent?.includes('auto')).toBe(true)
    expect(
      document.body.textContent?.includes(
        'Using the complete global Auto order (2 groups)'
      )
    ).toBe(true)
    expect(
      [
        ...document.querySelectorAll('[data-slot="global-auto-order-name"]'),
      ].map((item) => item.textContent)
    ).toEqual(['vip', 'default'])
    expect(findButton('Restore global Auto', true).disabled).toBe(true)

    changeInput(getControlByLabel('Name'), 'batch')
    changeInput(getControlByLabel('Quantity'), '2')
    fireEvent.click(findButton('Save changes', true))
    await waitFor(() => expect(createdPayloads).toHaveLength(2))

    expect(createdPayloads.length).toBe(2)
    expect(createdPayloads[0]?.name).toBe('batch')
    for (const payload of createdPayloads) {
      expect(payload.group).toBe('auto')
      expect(payload.auto_groups).toEqual([])
      expect(payload.cross_group_retry).toBe(true)
    }
  })

  test('preserves an unsaved custom order and mode after Auto to ordinary to Auto changes', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderDrawer()

    const autoOrderControl = getControlByLabel('Auto group order')
    const addGroupTrigger = autoOrderControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    if (!addGroupTrigger) {
      throw new Error('Expected Auto group order combobox')
    }
    selectComboboxOption(addGroupTrigger, 'Priority access')

    expect(
      document.querySelector('button[aria-label="Remove vip"]')
    ).toBeTruthy()
    expect(document.body.textContent?.includes('1 / 3 groups selected')).toBe(
      true
    )
    expect(findButton('Restore global Auto', true).disabled).toBe(false)

    const groupTrigger = getControlByLabel('Group')
    selectComboboxOption(groupTrigger, 'Standard access')
    expect(document.querySelector('button[aria-label="Remove vip"]')).toBe(null)
    selectComboboxOption(groupTrigger, 'Automatic routing')

    expect(
      document.querySelector('button[aria-label="Remove vip"]')
    ).toBeTruthy()
    expect(document.body.textContent?.includes('1 / 3 groups selected')).toBe(
      true
    )
    expect(findButton('Restore global Auto', true).disabled).toBe(false)

    changeInput(getControlByLabel('Name'), 'custom')
    fireEvent.click(findButton('Save changes', true))
    await waitFor(() => expect(createdPayloads).toHaveLength(1))
    expect(createdPayloads[0]?.auto_groups).toEqual(['vip'])
  })

  test('shows the fallback chain for an ordinary group and saves the switch', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderDrawer()

    selectComboboxOption(getControlByLabel('Group'), 'Standard access')
    const chain = () =>
      document.querySelector('[aria-label="Group execution chain"]')
        ?.textContent ?? ''
    await waitFor(() => expect(chain()).toContain('vip'))
    expect(chain()).toContain('default')

    const toggle = [...document.querySelectorAll('[role="switch"]')].find(
      (item) =>
        item
          .closest('[data-slot="form-item"]')
          ?.textContent?.includes('Fallback group retry')
    )
    if (!(toggle instanceof HTMLElement)) {
      throw new Error('Expected fallback group retry switch')
    }
    expect(toggle.getAttribute('aria-checked')).toBe('true')
    fireEvent.click(toggle)
    expect(chain()).not.toContain('vip')

    changeInput(getControlByLabel('Name'), 'fallback-off')
    fireEvent.click(findButton('Save changes', true))
    await waitFor(() => expect(createdPayloads).toHaveLength(1))
    expect(createdPayloads[0]?.group).toBe('default')
    expect(createdPayloads[0]?.cross_group_retry).toBe(false)
  })

  test('narrows the group picker to the chosen vendor and keeps auto', async () => {
    installApiFixtures([])
    await renderDrawer()
    const vendorFilter = await screen.findByRole('group', { name: 'Vendor' })
    fireEvent.click(
      within(vendorFilter).getByRole('button', { name: 'DeepSeek' })
    )
    fireEvent.click(screen.getByRole('combobox', { name: 'Group' }))
    expect(await screen.findByRole('option', { name: /^vip/ })).toBeTruthy()
    expect(screen.queryByRole('option', { name: /^default/ })).toBeNull()
    expect(screen.getByRole('option', { name: /^auto/ })).toBeTruthy()
  })

  test('clears a selected group that is not in the clicked vendor', async () => {
    installApiFixtures([])
    await renderDrawer()
    selectComboboxOption(getControlByLabel('Group'), 'Priority access')
    const vendorFilter = await screen.findByRole('group', { name: 'Vendor' })
    fireEvent.click(within(vendorFilter).getByRole('button', { name: '智谱' }))

    const trigger = screen.getByRole('combobox', { name: 'Group' })
    await waitFor(() => expect(trigger).toHaveTextContent('Select a group'))
    fireEvent.click(trigger)
    expect(await screen.findByRole('option', { name: /^default/ })).toBeTruthy()
    expect(screen.getByRole('option', { name: /^auto/ })).toBeTruthy()
    expect(screen.queryByRole('option', { name: /^vip/ })).toBeNull()
  })

  test('keeps a selected group that belongs to the clicked vendor', async () => {
    installApiFixtures([])
    await renderDrawer()
    selectComboboxOption(getControlByLabel('Group'), 'Priority access')
    const vendorFilter = await screen.findByRole('group', { name: 'Vendor' })
    fireEvent.click(
      within(vendorFilter).getByRole('button', { name: 'DeepSeek' })
    )
    expect(screen.getByRole('combobox', { name: 'Group' })).toHaveTextContent(
      'vip'
    )
  })

  test('shows a group without enabled models greyed out and unselectable', async () => {
    installApiFixtures([])
    await renderDrawer()
    await screen.findByRole('group', { name: 'Vendor' })
    fireEvent.click(screen.getByRole('combobox', { name: 'Group' }))
    const empty = await screen.findByRole('option', { name: /^empty/ })
    expect(empty).toHaveAttribute('aria-disabled', 'true')
    expect(empty).toHaveTextContent('Temporarily unavailable')
    fireEvent.click(empty)
    expect(
      screen.getByRole('combobox', { name: 'Group' })
    ).not.toHaveTextContent('empty')
  })

  test('keeps a group without own models selectable when a fallback group serves it', async () => {
    installApiFixtures([])
    await renderDrawer()
    await screen.findByRole('group', { name: 'Vendor' })
    fireEvent.click(screen.getByRole('combobox', { name: 'Group' }))
    const relay = await screen.findByRole('option', { name: /^relay/ })
    expect(relay).not.toHaveAttribute('aria-disabled', 'true')
    expect(relay).not.toHaveTextContent('Temporarily unavailable')
  })

  test('editing a key whose group has no enabled models keeps and flags that group', async () => {
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures([])
    apiClient.put = async (url, data) => {
      expect(url).toBe('/api/token/')
      updatedPayloads.push(data as Record<string, unknown>)
      return { data: { success: true, data: {} } }
    }
    await renderDrawer(emptyGroupKey)
    await screen.findByRole('group', { name: 'Vendor' })
    const trigger = screen.getByRole('combobox', { name: 'Group' })
    expect(trigger).toHaveTextContent('empty')
    expect(trigger).toHaveTextContent('Temporarily unavailable')

    fireEvent.click(findButton('Save changes', true))
    await waitFor(() => expect(updatedPayloads).toHaveLength(1))
    expect(updatedPayloads[0]?.group).toBe('empty')
  })
})
