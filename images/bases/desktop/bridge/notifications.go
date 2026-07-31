package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/godbus/dbus/v5"
)

const (
	maxApplicationRunes = 128
	maxTitleRunes       = 256
	maxBodyRunes        = 4096
	maxActiveItems      = 250
)

type notification struct {
	ID        uint32    `json:"id"`
	App       string    `json:"app,omitempty"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Urgency   string    `json:"urgency"`
	CreatedAt time.Time `json:"createdAt"`
	sequence  uint64
}

type notificationPage struct {
	Notifications []notification `json:"notifications"`
	NextCursor    string         `json:"nextCursor"`
}

type notificationQueue struct {
	mutex      sync.RWMutex
	generation string
	limit      int
	sequence   uint64
	items      []notification
}

func newNotificationQueue(limit int) (*notificationQueue, error) {
	if limit < 1 {
		return nil, errors.New("notification queue limit must be positive")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	return &notificationQueue{
		generation: hex.EncodeToString(random),
		limit:      limit,
	}, nil
}

func (queue *notificationQueue) append(item notification) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	queue.sequence++
	item.sequence = queue.sequence
	queue.items = append(queue.items, item)
	if overflow := len(queue.items) - queue.limit; overflow > 0 {
		copy(queue.items, queue.items[overflow:])
		queue.items = queue.items[:queue.limit]
	}
}

func (queue *notificationQueue) page(cursor string) (notificationPage, error) {
	queue.mutex.RLock()
	defer queue.mutex.RUnlock()

	after, err := queue.sequenceFromCursor(cursor)
	if err != nil {
		return notificationPage{}, err
	}
	items := make([]notification, 0, len(queue.items))
	for _, item := range queue.items {
		if item.sequence > after {
			items = append(items, item)
		}
	}
	return notificationPage{
		Notifications: items,
		NextCursor:    queue.cursor(queue.sequence),
	}, nil
}

func (queue *notificationQueue) sequenceFromCursor(cursor string) (uint64, error) {
	if cursor == "" {
		return 0, nil
	}
	generation, sequenceText, found := strings.Cut(cursor, ".")
	if !found || generation == "" || sequenceText == "" {
		return 0, errors.New("invalid notification cursor")
	}
	sequence, err := strconv.ParseUint(sequenceText, 10, 64)
	if err != nil {
		return 0, errors.New("invalid notification cursor")
	}
	if generation != queue.generation {
		return 0, nil
	}
	return sequence, nil
}

func (queue *notificationQueue) cursor(sequence uint64) string {
	return queue.generation + "." + strconv.FormatUint(sequence, 10)
}

type notificationServer struct {
	connection  *dbus.Conn
	queue       *notificationQueue
	mutex       sync.Mutex
	nextID      uint32
	active      map[uint32]struct{}
	activeOrder []uint32
}

func newNotificationServer(
	connection *dbus.Conn,
	queue *notificationQueue,
) *notificationServer {
	return &notificationServer{
		connection: connection,
		queue:      queue,
		active:     make(map[uint32]struct{}),
	}
}

func (server *notificationServer) GetCapabilities() ([]string, *dbus.Error) {
	return []string{"body"}, nil
}

func (server *notificationServer) GetServerInformation() (
	string,
	string,
	string,
	string,
	*dbus.Error,
) {
	return "Launcher Desktop Bridge", "PDP Architect", "0.1", "1.2", nil
}

func (server *notificationServer) Notify(
	appName string,
	replacesID uint32,
	_ string,
	summary string,
	body string,
	_ []string,
	hints map[string]dbus.Variant,
	_ int32,
) (uint32, *dbus.Error) {
	server.mutex.Lock()
	id := replacesID
	if _, exists := server.active[id]; id == 0 || !exists {
		server.nextID++
		if server.nextID == 0 {
			server.nextID++
		}
		id = server.nextID
		server.activeOrder = append(server.activeOrder, id)
	}
	server.active[id] = struct{}{}
	if len(server.activeOrder) > maxActiveItems {
		expired := server.activeOrder[0]
		server.activeOrder = server.activeOrder[1:]
		delete(server.active, expired)
	}
	server.mutex.Unlock()

	appName = truncateRunes(strings.TrimSpace(appName), maxApplicationRunes)
	summary = truncateRunes(strings.TrimSpace(summary), maxTitleRunes)
	if summary == "" {
		summary = appName
	}
	if summary == "" {
		summary = "Agent notification"
	}
	server.queue.append(notification{
		ID:        id,
		App:       appName,
		Title:     summary,
		Body:      truncateRunes(strings.TrimSpace(body), maxBodyRunes),
		Urgency:   notificationUrgency(hints),
		CreatedAt: time.Now().UTC(),
	})
	return id, nil
}

func (server *notificationServer) CloseNotification(id uint32) *dbus.Error {
	server.mutex.Lock()
	_, exists := server.active[id]
	delete(server.active, id)
	for index, activeID := range server.activeOrder {
		if activeID == id {
			server.activeOrder = append(
				server.activeOrder[:index],
				server.activeOrder[index+1:]...,
			)
			break
		}
	}
	server.mutex.Unlock()
	if !exists {
		return nil
	}
	if err := server.connection.Emit(
		notificationPath,
		notificationInterface+".NotificationClosed",
		id,
		uint32(3),
	); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func notificationUrgency(hints map[string]dbus.Variant) string {
	variant, exists := hints["urgency"]
	if !exists {
		return "normal"
	}
	urgency, ok := variant.Value().(byte)
	if !ok {
		return "normal"
	}
	switch urgency {
	case 0:
		return "low"
	case 2:
		return "critical"
	default:
		return "normal"
	}
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}
