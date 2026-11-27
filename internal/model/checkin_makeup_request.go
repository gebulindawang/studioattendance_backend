package model

import "time"

const (
	MakeupPending  = "pending"
	MakeupApproved = "approved"
	MakeupRejected = "rejected"
)

type CheckinMakeupRequest struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	RecordID   uint       `json:"record_id"`
	Reason     string     `json:"reason"`
	Status     string     `json:"status"`
	ReviewerID *int       `json:"reviewer_id,omitempty"`
	ReviewNote string     `json:"review_note"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
