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
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { expect, it, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'

import { Roles } from '..'

const role = {
  superuser: false,
  grants: {},
}

it('shows translated built-in roles and custom roles verbatim in zh', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        resources: [],
        roles: [
          {
            ...role,
            key: 'root',
            name: 'Root',
            description: 'Built-in root authorization role',
            built_in: true,
            superuser: true,
          },
          {
            ...role,
            key: 'ops',
            name: 'Admin',
            description: 'Built-in admin authorization role',
            built_in: false,
          },
        ],
      },
    },
  })
  const i18n = createInstance()
  await i18n.init({
    lng: 'zh',
    resources: { zh: { translation: zh.translation } },
    initAsync: false,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <Roles />
      </QueryClientProvider>
    </I18nextProvider>
  )

  expect(
    await screen.findByText(zh.translation['Built-in root authorization role'])
  ).toBeInTheDocument()
  expect(
    screen.getByRole('columnheader', { name: zh.translation['Role key'] })
  ).toBeInTheDocument()
  // A custom role's text is user data and is never translated.
  expect(
    screen.getByText('Built-in admin authorization role')
  ).toBeInTheDocument()
})
