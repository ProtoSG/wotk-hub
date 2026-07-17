interface EmptyStateProps {
  icon: React.ReactNode
  title: string
  description?: string
  action?: { label: string; onClick: () => void }
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-center space-y-2">
      <div className="text-muted-foreground mb-2 [&>svg]:h-8 [&>svg]:w-8">{icon}</div>
      <p className="font-medium text-foreground">{title}</p>
      {description && <p className="text-sm text-muted-foreground max-w-xs">{description}</p>}
      {action && (
        <button
          onClick={action.onClick}
          className="mt-2 text-sm text-primary hover:underline"
        >
          {action.label}
        </button>
      )}
    </div>
  )
}
