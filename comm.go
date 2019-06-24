package wserver

import (
	"time"
)

// one command, has several properties
// 1. async or sync
// 2. expire
// 3. need response or not

type CommObject struct {
	id string

	async bool

	start   time.Time
	timeout time.Duration

	request  *string
	response *string
	waitCH   chan struct{}
}
type CommRequest struct {
	id  string
	msg string
}
type CommResponse struct {
	id  string
	msg string
}
