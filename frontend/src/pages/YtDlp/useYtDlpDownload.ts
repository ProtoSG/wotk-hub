import { useEffect, useRef, useState } from 'react'
import { LOVE_MESSAGES, MESSAGE_INTERVAL_MS } from './loveMessages'

interface UseYtDlpDownloadOptions {
  /** The only thing that differs between the authenticated and public
   *  pages — which API call fetches the audio blob (token, if any, is
   *  already bound by the caller). */
  download: (url: string) => Promise<{ blob: Blob; filename: string }>
}

/**
 * Shared state + handlers for the YtDlp download form: URL input, loading
 * state, rotating "love message" while waiting, and the blob-download-and-
 * click boilerplate. Extracted from YtDlpPage/PublicYtDlpPage, which were
 * near-identical apart from which API hook backs `download`.
 */
export function useYtDlpDownload({ download }: UseYtDlpDownloadOptions) {
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [messageIndex, setMessageIndex] = useState(() => Math.floor(Math.random() * LOVE_MESSAGES.length))
  const intervalRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined)

  useEffect(() => {
    if (!loading) {
      clearInterval(intervalRef.current)
      return
    }
    intervalRef.current = setInterval(() => {
      setMessageIndex((i) => (i + 1) % LOVE_MESSAGES.length)
    }, MESSAGE_INTERVAL_MS)
    return () => clearInterval(intervalRef.current)
  }, [loading])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!url.trim() || loading) return

    setLoading(true)
    setError(null)
    try {
      const { blob, filename } = await download(url.trim())
      const objectUrl = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = objectUrl
      link.download = filename
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(objectUrl)
      setUrl('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo descargar el audio')
    } finally {
      setLoading(false)
    }
  }

  return { url, setUrl, loading, error, messageIndex, handleSubmit }
}
