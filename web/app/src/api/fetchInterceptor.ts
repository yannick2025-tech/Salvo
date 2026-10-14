import { sessionExpired } from '@/composables/useSessionExpired'

/**
 * Wrap the global fetch so that native fetch() calls (used by DashboardPage
 * polling) also trigger the session-expired dialog on body code 401.
 * Must be imported early in main.ts before any component calls fetch.
 */
function handleSessionExpired() {
  if (!sessionExpired.value) {
    sessionExpired.value = true
    localStorage.removeItem('salvo_token')
    localStorage.removeItem('salvo_user')
    localStorage.removeItem('salvo_permissions')
  }
}

const _origFetch = window.fetch
window.fetch = function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  return _origFetch.call(window, input, init).then((resp) => {
    // Clone so the original body is still consumable by the caller.
    const clone = resp.clone()
    clone.json().then((body) => {
      if (body?.code === 401) {
        handleSessionExpired()
      }
    }).catch(() => {
      // Response wasn't JSON — ignore (e.g. HTML, binary).
    })
    return resp
  })
}
