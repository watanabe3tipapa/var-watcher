import { createI18n } from 'vue-i18n'
import ja from './ja'
import en from './en'

export const LANG_KEY = 'varwatch.lang'
export const VALID_LANGS = ['ja', 'en'] as const
export type Lang = (typeof VALID_LANGS)[number]

export function normalizeLang(lang: string | null | undefined): Lang {
  return VALID_LANGS.includes(lang as Lang) ? (lang as Lang) : 'ja'
}

export const i18n = createI18n({
  legacy: false,
  locale: 'ja',
  fallbackLocale: 'en',
  messages: { ja, en },
  globalInjection: true,
})