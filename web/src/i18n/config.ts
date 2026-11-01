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
import i18n, { type BackendModule, type i18n as I18n } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import { convertDetectedLanguage } from './languages'
import zhCN from './locales/zh.json'

type LocaleModule = { default: { translation: Record<string, string> } }

// Simplified Chinese ships in the main bundle; every other language is its own
// chunk, downloaded only when a visitor uses it, so the first page load does
// not carry six unused translation files.
const lazyLocales: Record<string, () => Promise<LocaleModule>> = {
  en: () => import('./locales/en.json'),
  fr: () => import('./locales/fr.json'),
  ja: () => import('./locales/ja.json'),
  ru: () => import('./locales/ru.json'),
  vi: () => import('./locales/vi.json'),
  zhTW: () => import('./locales/zh-TW.json'),
}

// i18next takes the promise path only for a read declaring exactly two
// parameters; with any other arity it waits for a callback that never comes.
const lazyLocaleBackend: BackendModule = {
  type: 'backend',
  init() {},
  read: async (language: string, _namespace: string) => {
    const load = lazyLocales[language]
    return load ? (await load()).default.translation : {}
  },
}

// setupI18n initializes an i18next instance for the console: the app passes
// the global one, tests pass their own.
export function setupI18n(instance: I18n) {
  return instance
    .use(LanguageDetector)
    .use(initReactI18next)
    .use(lazyLocaleBackend)
    .init({
      resources: { zhCN },
      partialBundledLanguages: true,
      // Every locale file carries every key, so Chinese never needs the English
      // file; other languages keep English as the last resort.
      fallbackLng: { zhCN: [], zhTW: ['zhCN'], default: ['en'] },
      supportedLngs: ['en', 'zhCN', 'fr', 'ru', 'ja', 'vi', 'zhTW'],
      load: 'currentOnly',
      nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
      debug: import.meta.env.DEV,
      interpolation: {
        escapeValue: false, // not needed for react as it escapes by default
      },
      detection: {
        order: ['localStorage', 'navigator'],
        caches: ['localStorage'],
        // Browsers report `zh-CN`/`zh-TW`/`zh`; map them onto our `zhCN`/`zhTW`
        // codes (non-Chinese codes pass through for normal supportedLngs matching).
        convertDetectedLanguage,
      },
      // Switching language loads the new file before announcing the change, so
      // components never need to suspend for translations.
      react: { useSuspense: false },
    })
}

// Resolves once the detected language is loaded; render after it so the first
// paint is never in a fallback language.
export const i18nReady = setupI18n(i18n)

export default i18n
