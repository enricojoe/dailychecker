import type { ComponentProps } from 'react'
import { cn } from '@/lib/utils'

function Input({ className, ...props }: ComponentProps<'input'>) {
  return (
    <input
      className={cn(
        'flex h-9 w-full rounded-lg border border-input bg-background px-3 py-1 text-sm text-foreground shadow-sm transition-colors',
        'placeholder:text-muted-foreground',
        'focus-visible:outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
        'disabled:cursor-not-allowed disabled:opacity-50',
        'aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20',
        className
      )}
      {...props}
    />
  )
}

export { Input }
