import { showToast } from './toast'

// copyText copies text to the clipboard. Prefers the async Clipboard API and
// falls back to a hidden textarea + execCommand for non-secure contexts or
// when the document is not focused (e.g. after canvas interactions).
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.top = '-9999px'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.focus()
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      return ok
    } catch {
      return false
    }
  }
}

// copyWithToast copies text and shows a theme-aware confirmation toast.
export async function copyWithToast(text: string, message = '已复制'): Promise<void> {
  const ok = await copyText(text)
  showToast(ok ? message : '复制失败', ok ? 'success' : 'error', 1500)
}
