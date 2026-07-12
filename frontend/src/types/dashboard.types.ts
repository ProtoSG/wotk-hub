export interface MetricData {
  label: string
  /** Null while the underlying data is still loading — MetricCard renders a
   * small inline placeholder for just the value in that case, so the card
   * shell itself never has to wait on the fetch. */
  value: string | number | null
  change?: string
  trend?: 'up' | 'down' | 'neutral'
  icon?: string
  /** Marks this metric as the primary "at a glance" number, given extra visual weight. */
  primary?: boolean
}

export interface ChartDataPoint {
  name: string
  value: number
  value2?: number
}
