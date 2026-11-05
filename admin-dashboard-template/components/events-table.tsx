import { Calendar, MapPin, Users } from "lucide-react"

interface Event {
  id: string
  "event name": string
  location: string
  attendees: number
  "date:event date:start": string
}

interface EventsTableProps {
  events: Event[]
}

export function EventsTable({ events }: EventsTableProps) {
  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div
          key={event.id}
          className="flex items-center justify-between p-4 rounded-lg border bg-card hover:bg-accent/50 transition-colors"
        >
          <div className="flex-1 min-w-0">
            <h4 className="font-medium text-foreground truncate">{event["event name"]}</h4>
            <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
              <div className="flex items-center gap-1.5">
                <MapPin className="w-4 h-4" />
                <span>{event.location}</span>
              </div>
              <div className="flex items-center gap-1.5">
                <Calendar className="w-4 h-4" />
                <span>
                  {new Date(event["date:event date:start"]).toLocaleDateString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                  })}
                </span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2 ml-4">
            <Users className="w-4 h-4 text-muted-foreground" />
            <span className="font-semibold text-foreground">{event.attendees.toLocaleString()}</span>
          </div>
        </div>
      ))}
    </div>
  )
}
