package models

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	AadharNumber string `json:"aadhar_number"`
	Email        string `json:"email"`
	Role         string `json:"role"`
}

type Pass struct {
	ID           string  `json:"id"`
	VisitorName  string  `json:"visitor_name"`
	VisitorEmail string  `json:"visitor_email"`
	QRToken      string  `json:"qr_token"`
	Status       string  `json:"status"`
	HostUserID   string  `json:"host_user_id"`
	ValidUntil   *string `json:"valid_until"`
}

type AccessLog struct {
	ID       string `json:"id"`
	PassID   string `json:"pass_id"`
	GateID   string `json:"gate_id"`
	ScanTime string `json:"scan_time"` // RFC3339 naive UTC
	Status   string `json:"status"`
}
