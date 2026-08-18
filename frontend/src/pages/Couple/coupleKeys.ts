export function datesKey() {
  return ['couple', 'dates'] as const
}

export function photosKey(dateId: number) {
  return ['couple', 'dates', dateId, 'photos'] as const
}

export function galleryKey() {
  return ['couple', 'photos'] as const
}

export function poemsKey() {
  return ['couple', 'poems'] as const
}

export function todayPoemKey() {
  return ['couple', 'poems', 'today'] as const
}

export function videoFeedKey() {
  return ['couple', 'videos', 'feed'] as const
}
