export function LoadingScreen() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div
        role="status"
        aria-label="Loading"
        className="h-7 w-7 animate-spin rounded-full border-2 border-border border-t-primary"
      />
    </div>
  )
}
