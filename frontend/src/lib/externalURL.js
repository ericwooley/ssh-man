export async function openExternalURL(url) {
  if (typeof window !== 'undefined' && window.runtime?.BrowserOpenURL) {
    return window.runtime.BrowserOpenURL(url)
  }

  if (typeof window !== 'undefined') {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}
