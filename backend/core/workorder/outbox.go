package workorder

import (
	"fmt"
	"sync"
	"time"
)

type OutboxMessage struct {
	ID, Type, BindingID, Payload string
	Attempts                     int
	NextAttemptAt                time.Time
	LeaseToken                   string
	LeaseUntil                   time.Time
	Status                       string
	LastError                    string
}

const (
	OutboxPending    = "pending"
	OutboxDelivering = "delivering"
	OutboxDelivered  = "delivered"
	OutboxDeadLetter = "dead_letter"
)

type OutboxStore struct {
	mu       sync.Mutex
	next     uint64
	messages map[string]*OutboxMessage
}

func NewOutboxStore() *OutboxStore { return &OutboxStore{messages: map[string]*OutboxMessage{}} }
func (s *OutboxStore) Enqueue(message OutboxMessage) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	message.ID = fmt.Sprintf("outbox-%d", s.next)
	message.Status = OutboxPending
	message.NextAttemptAt = time.Now().UTC()
	s.messages[message.ID] = &message
	return message.ID
}
func (s *OutboxStore) Claim(now time.Time, lease time.Duration, limit int) ([]OutboxMessage, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := fmt.Sprintf("lease-%d", now.UnixNano())
	result := []OutboxMessage{}
	for _, message := range s.messages {
		if len(result) >= limit {
			break
		}
		if message.Status == OutboxDelivered || message.Status == OutboxDeadLetter || (message.Status == OutboxDelivering && message.LeaseUntil.After(now)) || message.NextAttemptAt.After(now) {
			continue
		}
		message.Status, message.LeaseToken, message.LeaseUntil = OutboxDelivering, token, now.Add(lease)
		result = append(result, *message)
	}
	return result, token
}
func (s *OutboxStore) Ack(id, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.messages[id]
	if !ok {
		return NewError(CodeWorkOrderNotFound, "outbox message not found")
	}
	if m.Status != OutboxDelivering || m.LeaseToken != token {
		return NewError(CodePermissionDenied, "outbox lease token is invalid")
	}
	m.Status, m.LeaseToken, m.LeaseUntil = OutboxDelivered, "", time.Time{}
	return nil
}
func (s *OutboxStore) Fail(id, token string, err error, maxAttempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.messages[id]
	if !ok {
		return NewError(CodeWorkOrderNotFound, "outbox message not found")
	}
	if m.Status != OutboxDelivering || m.LeaseToken != token {
		return NewError(CodePermissionDenied, "outbox lease token is invalid")
	}
	m.Attempts++
	m.LastError = err.Error()
	m.LeaseToken, m.LeaseUntil = "", time.Time{}
	if m.Attempts >= maxAttempts {
		m.Status = OutboxDeadLetter
	} else {
		m.Status, m.NextAttemptAt = OutboxPending, time.Now().UTC().Add(time.Duration(m.Attempts)*time.Second)
	}
	return nil
}
