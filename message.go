package queue

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

var (
	// TODO: to generic errors, and specific comment in errors.Wrapf()
	ErrNotFoundMessage     = errors.New("not found message")
	ErrNotMatchTypeMessage = errors.New("not match type message")
)

// globalMessage Global message key-value store object
var globalMessage *DatastoreMessage = NewDatastoreMessage()

type messageState int

const (
	_ messageState = iota
	stateWait
	stateDeliver
	stateAck
)

func (s messageState) String() string {
	switch s {
	case stateWait:
		return "Waiting"
	case stateDeliver:
		return "Delivered"
	case stateAck:
		return "Ack"
	default:
		return "Non define"
	}
}

// Message is data object
type Message struct {
	ID          string
	Topic       Topic
	Data        []byte
	Attributes  *Attributes
	States      *states
	PublishedAt time.Time
	DeliveredAt time.Time
}

func makeMessageID() string {
	return uuid.NewV1().String()
}

func NewMessage(id string, topic Topic, data []byte, attr map[string]string, subs []*Subscription) *Message {
	m := &Message{
		ID:         id,
		Data:       data,
		Attributes: newAttributes(attr),
		Topic:      topic,
		States: &states{
			list: make(map[string]messageState),
		},
		PublishedAt: time.Now(),
	}
	for _, sub := range subs {
		m.States.add(sub.name)
	}
	return m
}

func (m *Message) Ack(subID string) {
	m.States.ack(subID)
}

func (m *Message) Deliver(subID string) {
	// TODO: need DeliveredAt in each Subscriptions?
	if m.States.deliver(subID) {
		m.DeliveredAt = time.Now()
	}
}

func (m *Message) Readable(id string, timeout time.Duration) bool {
	state, ok := m.States.get(id)
	if !ok || state == stateAck {
		return false
	}

	// not readable between deliver and ack
	if state == stateDeliver {
		return time.Now().Sub(m.DeliveredAt) > timeout
	}
	return true
}

// states repsents Subscriptions and Ack map.
type states struct {
	list map[string]messageState
	mu   sync.RWMutex
}

func (s *states) String() string {
	strs := make([]string, 0)
	for k, v := range s.list {
		strs = append(strs, fmt.Sprintf("%s:%v", k, v))
	}
	return strings.Join(strs, ", ")
}

func (s *states) ack(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.list[id]
	if !ok || state != stateDeliver {
		return false
	}
	s.list[id] = stateAck
	return true
}

func (s *states) deliver(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.list[id]
	if !ok || state != stateWait {
		return false
	}
	s.list[id] = stateDeliver
	return true
}

func (s *states) add(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.list[id] = stateWait
}

func (s *states) get(id string) (messageState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.list[id]
	return state, ok
}
