import { Button } from '@/components/ui/button'

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL as string | undefined

export default function App() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-background p-8 text-foreground">
      <div className="flex flex-col items-center gap-2 text-center">
        <h1 className="text-4xl font-bold tracking-tight">DailyChecker</h1>
        <p className="text-muted-foreground text-sm">
          Multi-user daily activity tracker — scaffold placeholder
        </p>
      </div>

      <div className="rounded-lg border border-border bg-card px-6 py-4 text-sm text-card-foreground shadow-sm">
        <span className="font-medium text-muted-foreground">Backend: </span>
        <code className="font-mono">
          {apiBaseUrl ?? <span className="text-destructive">VITE_API_BASE_URL not set</span>}
        </code>
      </div>

      {/* Proves the shadcn/ui Button + Tailwind stack is wired up */}
      <Button variant="default" size="default">
        Get Started
      </Button>
    </main>
  )
}
