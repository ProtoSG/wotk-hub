export function exercisesKey(q: string, muscle: string, equipment: string, page: number) {
  return ['gym', 'exercises', q, muscle, equipment, page] as const
}

export function exerciseFiltersKey() {
  return ['gym', 'exercises', 'filters'] as const
}

export function activeSessionKey() {
  return ['gym', 'sessions', 'active'] as const
}

export function sessionsKey() {
  return ['gym', 'sessions'] as const
}

export function lastSetsKey(exerciseId: number) {
  return ['gym', 'exercises', exerciseId, 'last-sets'] as const
}
