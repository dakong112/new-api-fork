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
import { createInstance } from 'i18next'
import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import { setupI18n } from '../config'

// Each test sets up its own instance; the test setup already owns the global.
async function loadI18n(storedLanguage: string) {
  localStorage.setItem('i18nextLng', storedLanguage)
  const i18n = createInstance()
  await setupI18n(i18n)
  return i18n
}

describe('lazy locale loading', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    localStorage.clear()
  })

  test('a Chinese visitor gets Chinese text without downloading English', async () => {
    const i18n = await loadI18n('zhCN')

    expect(i18n.t('auth.resetPasswordConfirm.backToLogin')).toBe('返回登录')
    expect(i18n.hasResourceBundle('en', 'translation')).toBe(false)
  })

  test('switching to English loads the English file before the change', async () => {
    const i18n = await loadI18n('zhCN')

    await i18n.changeLanguage('en')

    expect(i18n.resolvedLanguage).toBe('en')
    expect(i18n.t('auth.resetPasswordConfirm.backToLogin')).toBe(
      'Return to login'
    )
  })

  test('a stored non-Chinese language is loaded before the first render', async () => {
    const i18n = await loadI18n('en')

    expect(i18n.t('auth.resetPasswordConfirm.backToLogin')).toBe(
      'Return to login'
    )
  })
})
