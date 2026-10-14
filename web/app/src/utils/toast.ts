// showToast displays a transient theme-aware toast at the bottom-right corner.
// Colors use the global CSS variables, so it adapts to dark/light themes.

export type ToastType = 'info' | 'success' | 'error'

let styleEl: HTMLStyleElement | null = null

function ensureStyle() {
  if (styleEl) return
  styleEl = document.createElement('style')
  styleEl.textContent = `
.app-toast { position: fixed; bottom: 24px; right: 24px; padding: 10px 20px; border-radius: var(--radius-md); font-size: 13px; z-index: 2000; max-width: 60vw; animation: appToastIn 0.3s ease; }
.app-toast.info { background: var(--accent-primary); color: #fff; }
.app-toast.success { background: var(--accent-success); color: #fff; }
.app-toast.error { background: var(--accent-danger, #e74c3c); color: #fff; }
@keyframes appToastIn { from { transform: translateY(20px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }`
  document.head.appendChild(styleEl)
}

export function showToast(message: string, type: ToastType = 'info', duration = 2500) {
  ensureStyle()
  // Only one toast at a time; rapid clicks replace the previous one.
  document.querySelectorAll('.app-toast').forEach((el) => el.remove())
  const el = document.createElement('div')
  el.className = `app-toast ${type}`
  el.textContent = message
  document.body.appendChild(el)
  window.setTimeout(() => el.remove(), duration)
}
