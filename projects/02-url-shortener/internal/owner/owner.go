package owner

import "time"

type Owner struct {
	ID        string    `json:"id"`
	Quota     int       `json:"quota"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}
