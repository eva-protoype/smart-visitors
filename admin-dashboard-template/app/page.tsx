"use client"

import { useEffect, useState } from "react"
import { Calendar, TrendingUp, Users, CalendarClock } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

interface Event {
  id: string
  "event name": string
  location: string
  attendees: number
  "date:event date:start": string
  "date:date added:start": string
}

function StatusDot({ variant }: { variant: "positive" | "warning" | "negative" }) {
  const colors = {
    positive: "bg-emerald-500",
    warning: "bg-amber-500",
    negative: "bg-red-500",
  }
  return <span className={`inline-block w-2 h-2 rounded-full ${colors[variant]}`} />
}

function Gauge({ value, size, showValue }: { value: number; size: string; showValue: boolean }) {
  const sizeClasses = size === "large" ? "w-32 h-32" : "w-24 h-24"
  return (
    <div className={`relative ${sizeClasses} flex items-center justify-center`}>
      <svg className="transform -rotate-90 w-full h-full" viewBox="0 0 100 100">
        <circle cx="50" cy="50" r="40" fill="none" stroke="rgb(229, 231, 235)" strokeWidth="8" />
        <circle
          cx="50"
          cy="50"
          r="40"
          fill="none"
          stroke="rgb(59, 130, 246)"
          strokeWidth="8"
          strokeDasharray={`${2 * Math.PI * 40}`}
          strokeDashoffset={`${2 * Math.PI * 40 * (1 - value / 100)}`}
          strokeLinecap="round"
          className="transition-all duration-500"
        />
      </svg>
      {showValue && (
        <div className="absolute inset-0 flex items-center justify-center">
          <span className="text-2xl font-semibold">{value.toFixed(1)}%</span>
        </div>
      )}
    </div>
  )
}

export default function EventsDashboard() {
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchEvents() {
      try {
        const response = await fetch("/api/events")
        const data = await response.json()
        setEvents(data.events || [])
      } catch (error) {
        console.error("[v0] Error fetching events:", error)
      } finally {
        setLoading(false)
      }
    }

    fetchEvents()
  }, [])

  const GOAL = 5000000
  const TARGET_DATE = new Date("2026-12-31")
  const START_DATE = new Date("2025-11-25")
  const TODAY = new Date()

  const pastEvents = events.filter((e) => new Date(e["date:event date:start"]) <= TODAY)
  const upcomingEvents = events.filter((e) => new Date(e["date:event date:start"]) > TODAY)

  const weekStart = new Date("2025-12-07")
  const weekEnd = new Date("2025-12-13")
  weekEnd.setHours(23, 59, 59, 999)

  const eventsThisWeek = events.filter((e) => {
    const eventDate = new Date(e["date:event date:start"])
    return eventDate >= weekStart && eventDate <= weekEnd
  })

  const attendeesThisWeek = eventsThisWeek.reduce((sum, e) => sum + (e.attendees || 0), 0)

  const scheduledThisWeek = events.filter((e) => {
    const addedDate = new Date(e["date:date added:start"])
    return addedDate >= weekStart && addedDate <= weekEnd
  }).length

  const totalAttendees = pastEvents.reduce((sum, e) => sum + (e.attendees || 0), 0)
  const totalEvents = pastEvents.length
  const progressPercent = ((totalAttendees / GOAL) * 100).toFixed(1)

  const scheduledAttendees = upcomingEvents.reduce((sum, e) => sum + (e.attendees || 0), 0)
  const projectedTotal = totalAttendees + scheduledAttendees

  const daysRemaining = Math.ceil((TARGET_DATE.getTime() - TODAY.getTime()) / (1000 * 60 * 60 * 24))

  if (loading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <Skeleton className="w-[300px] h-10" />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="mx-auto max-w-7xl px-6 py-12">
        {/* Header */}
        <div className="mb-12">
          <div className="flex items-center gap-3 mb-3">
            <div className="flex items-center justify-center w-10 h-10 rounded-lg bg-primary text-primary-foreground">
              <Calendar className="w-5 h-5" />
            </div>
            <h1 className="text-3xl font-semibold text-foreground">v0 IRL Events Dashboard</h1>
          </div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <StatusDot variant="positive" />
            <span>Live</span>
            <span className="text-muted-foreground/50">•</span>
            <span>Weekly performance report - Thursday, December 11, 2025</span>
          </div>
        </div>

        {/* Weekly Update */}
        <section className="mb-12">
          <h2 className="text-lg font-medium mb-6 text-foreground">Weekly Update</h2>
          <div className="grid gap-4 md:grid-cols-3">
            <div className="border border-border rounded-lg p-6 bg-card hover:border-border/80 transition-colors">
              <div className="flex items-center gap-2 text-sm text-muted-foreground mb-3">
                <Calendar className="w-4 h-4 text-blue-500" />
                <span>Events This Week</span>
              </div>
              <div className="text-4xl font-semibold mb-2 text-foreground">{eventsThisWeek.length}</div>
              <div className="text-sm text-muted-foreground">Scheduled or completed (Dec 7 - Dec 13)</div>
            </div>

            <div className="border border-border rounded-lg p-6 bg-card hover:border-border/80 transition-colors">
              <div className="flex items-center gap-2 text-sm text-muted-foreground mb-3">
                <Users className="w-4 h-4 text-purple-500" />
                <span>Attendees This Week</span>
              </div>
              <div className="text-4xl font-semibold mb-2 text-foreground">{attendeesThisWeek.toLocaleString()}</div>
              <div className="text-sm text-muted-foreground">Dec 7 - Dec 13</div>
            </div>

            <div className="border border-border rounded-lg p-6 bg-card hover:border-border/80 transition-colors">
              <div className="flex items-center gap-2 text-sm text-muted-foreground mb-3">
                <CalendarClock className="w-4 h-4 text-amber-500" />
                <span>Upcoming Events Scheduled</span>
              </div>
              <div className="text-4xl font-semibold mb-2 text-foreground">{upcomingEvents.length}</div>
              <div className="text-sm text-muted-foreground">+{scheduledThisWeek} added this week</div>
            </div>
          </div>
        </section>

        {/* All Time Engagement */}
        <section className="mb-12">
          <h2 className="text-lg font-medium mb-6 text-foreground">All Time Engagement</h2>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="border border-border rounded-lg p-6 bg-card">
              <div className="flex items-center gap-2 text-sm text-muted-foreground mb-3">
                <Users className="w-4 h-4 text-purple-500" />
                <span>Total Attendees</span>
              </div>
              <div className="text-4xl font-semibold mb-2 text-foreground">{totalAttendees.toLocaleString()}</div>
              <div className="text-sm text-muted-foreground">Across all completed events</div>
            </div>

            <div className="border border-border rounded-lg p-6 bg-card">
              <div className="flex items-center gap-2 text-sm text-muted-foreground mb-3">
                <Calendar className="w-4 h-4 text-emerald-500" />
                <span>Total Events</span>
              </div>
              <div className="text-4xl font-semibold mb-2 text-foreground">{totalEvents}</div>
              <div className="text-sm text-muted-foreground">Successfully completed</div>
            </div>
          </div>
        </section>

        {/* 2026 Goal */}
        <section className="mb-12">
          <div className="border border-border rounded-lg p-8 bg-card">
            <div className="border-l-4 border-primary pl-6 mb-8">
              <h2 className="text-2xl font-semibold mb-2 text-foreground">2026 Goal</h2>
              <p className="text-lg text-muted-foreground mb-1">5,000,000 attendees engaged</p>
              <p className="text-sm text-muted-foreground/70">Event tracking started: Nov 25, 2025</p>
            </div>

            {/* Metrics Grid */}
            <div className="grid gap-6 md:grid-cols-3 mb-12">
              <div className="border border-border rounded-lg p-6 bg-muted/30">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground uppercase tracking-wide mb-4">
                  <Users className="w-3.5 h-3.5" />
                  People Reached
                </div>
                <div className="text-3xl font-semibold mb-4 text-foreground">{totalAttendees.toLocaleString()}</div>
                <div className="space-y-2">
                  <div className="h-1.5 w-full bg-muted rounded-full overflow-hidden">
                    <div
                      className="h-full bg-primary transition-all duration-500"
                      style={{ width: `${Math.min(Number.parseFloat(progressPercent), 100)}%` }}
                    />
                  </div>
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-medium text-foreground">{progressPercent}%</span>
                    <span className="text-muted-foreground">+0 from scheduled events</span>
                  </div>
                </div>
              </div>

              <div className="border border-border rounded-lg p-6 bg-muted/30">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground uppercase tracking-wide mb-4">
                  <TrendingUp className="w-3.5 h-3.5" />
                  Projected Total
                </div>
                <div className="text-3xl font-semibold mb-4 text-foreground">{projectedTotal.toLocaleString()}</div>
                <div className="space-y-2">
                  <div className="h-1.5 w-full bg-muted rounded-full overflow-hidden">
                    <div
                      className="h-full bg-primary transition-all duration-500"
                      style={{ width: `${Math.min((projectedTotal / GOAL) * 100, 100)}%` }}
                    />
                  </div>
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-medium text-foreground">{((projectedTotal / GOAL) * 100).toFixed(1)}%</span>
                    <span className="text-muted-foreground">Need additional events</span>
                  </div>
                </div>
              </div>

              <div className="border border-border rounded-lg p-6 bg-muted/30">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground uppercase tracking-wide mb-4">
                  <Calendar className="w-3.5 h-3.5" />
                  Days Remaining
                </div>
                <div className="text-3xl font-semibold mb-4 text-foreground">{daysRemaining}</div>
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">days until deadline</p>
                  <p className="text-xs text-muted-foreground/70">Target: December 31, 2026</p>
                </div>
              </div>
            </div>

            {/* Goal Progress with Gauge */}
            <div className="border-t border-border pt-8">
              <h3 className="text-base font-medium mb-2 text-foreground">Goal Progress Overview</h3>
              <p className="text-sm text-muted-foreground mb-8">
                Tracking towards 5,000,000 attendees by December 31, 2026
              </p>
              <div className="grid gap-12 md:grid-cols-2 max-w-3xl mx-auto">
                <div className="flex flex-col items-center">
                  <Gauge value={Number.parseFloat(progressPercent)} size="large" showValue />
                  <div className="text-center mt-4">
                    <div className="font-medium text-foreground">Current Progress</div>
                    <div className="text-sm text-muted-foreground mt-1">
                      {totalAttendees.toLocaleString()} of {GOAL.toLocaleString()} attendees
                    </div>
                  </div>
                </div>
                <div className="flex flex-col items-center">
                  <Gauge value={(projectedTotal / GOAL) * 100} size="large" showValue />
                  <div className="text-center mt-4">
                    <div className="font-medium text-foreground">With Scheduled Events</div>
                    <div className="text-sm text-muted-foreground mt-1">
                      {projectedTotal.toLocaleString()} projected attendees
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Event Pipeline */}
        <section>
          <div className="border border-border rounded-lg p-8 bg-card">
            <div className="mb-6">
              <h2 className="text-lg font-medium mb-1 text-foreground">Event Pipeline & Upcoming Schedule</h2>
              <p className="text-sm text-muted-foreground">Future events scheduled and pending approval</p>
            </div>

            <div className="grid gap-4 md:grid-cols-3 mb-8">
              <div className="border border-border rounded-lg p-5 bg-muted/30">
                <div className="flex items-center gap-3">
                  <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-blue-500/10">
                    <Calendar className="w-4 h-4 text-blue-500" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground mb-0.5">Total Scheduled</p>
                    <p className="text-2xl font-semibold text-foreground">{upcomingEvents.length}</p>
                  </div>
                </div>
              </div>

              <div className="border border-border rounded-lg p-5 bg-muted/30">
                <div className="flex items-center gap-3">
                  <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-purple-500/10">
                    <CalendarClock className="w-4 h-4 text-purple-500" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground mb-0.5">Added This Week</p>
                    <p className="text-2xl font-semibold text-foreground">{scheduledThisWeek}</p>
                  </div>
                </div>
              </div>

              <div className="border border-border rounded-lg p-5 bg-muted/30">
                <div className="flex items-center gap-3">
                  <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-amber-500/10">
                    <TrendingUp className="w-4 h-4 text-amber-500" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground mb-0.5">Projected Attendees</p>
                    <p className="text-2xl font-semibold text-foreground">{scheduledAttendees.toLocaleString()}</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Upcoming Events Table */}
            <div className="border-t border-border pt-6">
              <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-4">
                Upcoming Events
              </h4>
              {upcomingEvents.length > 0 ? (
                <div className="border border-border rounded-lg overflow-hidden">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Event</TableHead>
                        <TableHead>Location</TableHead>
                        <TableHead>Date</TableHead>
                        <TableHead className="text-right">Attendees</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {upcomingEvents.slice(0, 10).map((event) => (
                        <TableRow key={event.id}>
                          <TableCell>
                            <div className="font-medium text-foreground">{event["event name"]}</div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary">{event.location}</Badge>
                          </TableCell>
                          <TableCell>
                            <span className="text-sm text-muted-foreground">
                              {new Date(event["date:event date:start"]).toLocaleDateString("en-US", {
                                month: "short",
                                day: "numeric",
                                year: "numeric",
                              })}
                            </span>
                          </TableCell>
                          <TableCell className="text-right">
                            <span className="font-medium text-foreground">
                              {event.attendees?.toLocaleString() || "0"}
                            </span>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground py-8 text-center">No upcoming events scheduled</p>
              )}
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
