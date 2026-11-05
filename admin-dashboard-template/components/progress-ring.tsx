"use client"

interface ProgressRingProps {
  value: number
  label: string
  subtitle: string
}

export function ProgressRing({ value, label, subtitle }: ProgressRingProps) {
  const radius = 70
  const circumference = 2 * Math.PI * radius
  const strokeDashoffset = circumference - (value / 100) * circumference

  return (
    <div className="flex flex-col items-center justify-center py-8">
      <div className="relative w-48 h-48">
        <svg className="w-full h-full transform -rotate-90" viewBox="0 0 160 160">
          {/* Background circle */}
          <circle cx="80" cy="80" r={radius} fill="none" stroke="hsl(var(--muted))" strokeWidth="12" />
          {/* Progress circle */}
          <circle
            cx="80"
            cy="80"
            r={radius}
            fill="none"
            stroke="hsl(var(--primary))"
            strokeWidth="12"
            strokeDasharray={circumference}
            strokeDashoffset={strokeDashoffset}
            strokeLinecap="round"
            className="transition-all duration-1000 ease-out"
          />
        </svg>
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="text-center">
            <div className="text-4xl font-bold text-primary">{value.toFixed(1)}%</div>
          </div>
        </div>
      </div>
      <div className="text-center mt-6 space-y-1">
        <div className="text-sm font-semibold text-foreground">{label}</div>
        <p className="text-sm text-muted-foreground max-w-xs">{subtitle}</p>
      </div>
    </div>
  )
}
