function Gauge({ value, size, showValue }: { value: number; size: string; showValue: boolean }) {
  const sizeClasses = size === "large" ? "w-32 h-32" : "w-24 h-24"
  return (
    <div className={`relative ${sizeClasses} flex items-center justify-center`}>
      <svg className="transform -rotate-90 w-full h-full" viewBox="0 0 100 100">
        {/* Background circle - updated for dark mode */}
        <circle cx="50" cy="50" r="40" fill="none" stroke="hsl(var(--muted))" strokeWidth="8" />
        {/* Progress circle - updated to use primary color */}
        <circle
          cx="50"
          cy="50"
          r="40"
          fill="none"
          stroke="hsl(var(--primary))"
          strokeWidth="8"
          strokeDasharray={`${2 * Math.PI * 40}`}
          strokeDashoffset={`${2 * Math.PI * 40 * (1 - value / 100)}`}
          strokeLinecap="round"
          className="transition-all duration-500"
        />
      </svg>
      {showValue && (
        <div className="absolute inset-0 flex items-center justify-center">
          <span className="text-2xl font-semibold text-foreground">{value.toFixed(1)}%</span>
        </div>
      )}
    </div>
  )
}

export { Gauge }
