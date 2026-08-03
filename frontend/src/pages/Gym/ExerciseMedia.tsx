import { useRef, useState } from 'react'
import { Dumbbell } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Exercise } from '@/types/gym.types'

interface ExerciseMediaProps {
  exercise: Exercise
  className?: string
  /** Plays the clip on its own — for the detail view, where it is the subject. */
  autoPlay?: boolean
}

/**
 * Catalog thumbnail. Two thirds of the catalog ships an .mp4 rather than a
 * still, so videos get a frame of their own instead of falling back to the
 * icon:
 *
 * - `preload="metadata"` plus the `#t=0.1` media fragment makes the browser
 *   decode and paint one frame, which stands in for the `poster` the source
 *   never provides. Without the fragment the element renders blank until
 *   something plays it.
 * - Hovering plays the clip muted and looping — the movement is the point of
 *   a demo — and leaving resets it to the still frame. Touch devices never
 *   fire hover, so they simply keep the frame and cost one metadata request.
 *
 * The media is hosted on a third-party bucket, so any failure (404, blocked
 * request, unsupported codec) falls back to the icon rather than leaving a
 * broken box in the row.
 */
export default function ExerciseMedia({ exercise, className, autoPlay }: ExerciseMediaProps) {
  const [failed, setFailed] = useState(false)
  const videoRef = useRef<HTMLVideoElement>(null)

  const hasMedia = exercise.mediaUrl !== '' && !failed

  // The caller's className goes LAST so twMerge lets it win — the detail view
  // overrides object-cover with object-contain, and an earlier default would
  // silently beat it.
  const box = (...extra: string[]) =>
    cn('shrink-0 overflow-hidden rounded-md bg-muted', ...extra, className)

  if (!hasMedia) {
    return (
      <div className={box('flex items-center justify-center text-muted-foreground')}>
        <Dumbbell className="h-5 w-5" />
      </div>
    )
  }

  if (exercise.mediaType === 'video') {
    return (
      <video
        ref={videoRef}
        // The fragment is what produces the still frame; it is not a cache
        // buster and must stay on the URL.
        src={`${exercise.mediaUrl}#t=0.1`}
        preload="metadata"
        autoPlay={autoPlay}
        muted
        loop
        playsInline
        disablePictureInPicture
        aria-hidden
        onError={() => setFailed(true)}
        onMouseEnter={() => {
          if (autoPlay) return
          // play() rejects if the element is detached mid-hover; nothing to
          // recover from, the still frame stays on screen.
          void videoRef.current?.play().catch(() => {})
        }}
        onMouseLeave={() => {
          if (autoPlay) return
          const video = videoRef.current
          if (!video) return
          video.pause()
          video.currentTime = 0.1
        }}
        className={box('object-cover')}
      />
    )
  }

  return (
    <img
      src={exercise.mediaUrl}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
      className={box('object-cover')}
    />
  )
}
