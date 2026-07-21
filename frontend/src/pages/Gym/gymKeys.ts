export function exercisesKey(q: string, muscle: string, equipment: string, page: number) {
  return ['gym', 'exercises', q, muscle, equipment, page] as const
}

export function exerciseFiltersKey() {
  return ['gym', 'exercises', 'filters'] as const
}
