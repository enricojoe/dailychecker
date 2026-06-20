/**
 * Centralised token storage.
 *
 * Both tokens live in localStorage so the session survives a hard page
 * refresh (M7 DoD requirement).
 *
 * XSS trade-off: localStorage is accessible to any JS on the page.
 * This is acceptable for v1 and is documented for M9 revisit (httpOnly
 * cookies or token binding).
 */

const ACCESS_KEY = 'dc_access'
const REFRESH_KEY = 'dc_refresh'

export const tokenStore = {
  getAccess: (): string | null => localStorage.getItem(ACCESS_KEY),
  getRefresh: (): string | null => localStorage.getItem(REFRESH_KEY),

  set: (access: string, refresh: string): void => {
    localStorage.setItem(ACCESS_KEY, access)
    localStorage.setItem(REFRESH_KEY, refresh)
  },

  clear: (): void => {
    localStorage.removeItem(ACCESS_KEY)
    localStorage.removeItem(REFRESH_KEY)
  },
}
