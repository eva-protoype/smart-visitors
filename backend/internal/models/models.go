package models

import "time"

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleHost  UserRole = "host"
	RoleGuard UserRole = "guard"
)

type PassStatus string

const (
	PassDraft     PassStatus = "draft"
	PassActive    PassStatus = "active"
	PassUsed      PassStatus = "used"
	PassExpired   PassStatus = "expired"
	PassRevoked   PassStatus = "revoked"
	PassShredded  PassStatus = "shredded"
)

type PassMode string

const (
	PassOneTime PassMode = "one_time"
	PassWindow  PassMode = "window"
)

type ScanDirection string

const (
	ScanEntry ScanDirection = "entry"
	ScanExit  ScanDirection = "exit"
)

type ScanOutcome string

const (
	ScanApproved ScanOutcome = "approved"
	ScanDenied   ScanOutcome = "denied"
)

type User struct {
	ID                string     `json:"id"`
	Role              UserRole   `json:"role"`
	Username          string     `json:"username"`
	Name              string     `json:"name"`
	Email             *string    `json:"email,omitempty"`
	Phone             *string    `json:"phone,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	MustResetPassword bool       `json:"must_reset_password"`
}

type Visitor struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Phone     *string    `json:"phone,omitempty"`
	PhotoRef  *string    `json:"photo_ref,omitempty"`
	ConsentAt *time.Time `json:"consent_at,omitempty"`
}

type Pass struct {
	ID         string     `json:"id"`
	Status     PassStatus `json:"status"`
	Mode       PassMode   `json:"mode"`
	VisitorID  string     `json:"visitor_id"`
	HostID     string     `json:"host_id"`
	GateID     *string    `json:"gate_id,omitempty"`
	Purpose    string     `json:"purpose"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidUntil time.Time  `json:"valid_until"`
	CreatedVia string     `json:"created_via"`
}

type AccessLog struct {
	Sequence  int64         `json:"seq"`
	ID        string        `json:"id"`
	PassID    *string       `json:"pass_id,omitempty"`
	GateID    *string       `json:"gate_id,omitempty"`
	GuardID   *string       `json:"guard_id,omitempty"`
	ScanTime  time.Time     `json:"scan_time"`
	Direction ScanDirection `json:"direction"`
	Outcome   ScanOutcome   `json:"outcome"`
	Reason    string        `json:"reason"`
	Hash      string        `json:"hash"`
}

type AuditEvent struct {
	Sequence int64          `json:"seq"`
	ID       string         `json:"id"`
	ActorID  *string        `json:"actor_id,omitempty"`
	Action   string         `json:"action"`
	Target   string         `json:"target"`
	Reason   *string        `json:"reason,omitempty"`
	Metadata map[string]any `json:"meta"`
	Hash     string         `json:"hash"`
}
