const DEFAULT_FILENAME = 'audio.mp3'

/** Extracts the filename from a Content-Disposition header, preferring the UTF-8
 *  `filename*` param (RFC 5987) over the ASCII-only `filename` fallback. */
export function parseFilename(contentDisposition: string | undefined): string {
  if (!contentDisposition) return DEFAULT_FILENAME

  const utf8Match = /filename\*=UTF-8''([^;]+)/i.exec(contentDisposition)
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]).trim() || DEFAULT_FILENAME
    } catch {
      // fall through to the ASCII filename param below
    }
  }

  const match = /filename="?([^";]+)"?/i.exec(contentDisposition)
  return match?.[1]?.trim() || DEFAULT_FILENAME
}
