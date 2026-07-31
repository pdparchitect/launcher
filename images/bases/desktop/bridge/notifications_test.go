package main

import (
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestNotificationQueueCursorAndLimit(t *testing.T) {
	queue, err := newNotificationQueue(2)
	if err != nil {
		t.Fatal(err)
	}
	queue.generation = "generation"
	queue.append(notification{ID: 1, Title: "one"})
	first, err := queue.page("")
	if err != nil {
		t.Fatal(err)
	}
	if first.NextCursor != "generation.1" || len(first.Notifications) != 1 {
		t.Fatalf("first page = %#v", first)
	}

	queue.append(notification{ID: 2, Title: "two"})
	queue.append(notification{ID: 3, Title: "three"})
	next, err := queue.page(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if next.NextCursor != "generation.3" ||
		len(next.Notifications) != 2 ||
		next.Notifications[0].ID != 2 ||
		next.Notifications[1].ID != 3 {
		t.Fatalf("next page = %#v", next)
	}
}

func TestNotificationQueueTreatsAnotherGenerationAsRestart(t *testing.T) {
	queue, err := newNotificationQueue(2)
	if err != nil {
		t.Fatal(err)
	}
	queue.generation = "new"
	queue.append(notification{ID: 1, Title: "after restart"})

	page, err := queue.page("old.42")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Notifications) != 1 || page.Notifications[0].ID != 1 {
		t.Fatalf("page after restart = %#v", page)
	}
}

func TestNotificationServerReplacesActiveNotification(t *testing.T) {
	queue, err := newNotificationQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	server := newNotificationServer(nil, queue)
	firstID, dbusError := server.Notify(
		"Agent",
		0,
		"",
		"First",
		"Working",
		nil,
		map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(2))},
		-1,
	)
	if dbusError != nil {
		t.Fatal(dbusError)
	}
	secondID, dbusError := server.Notify(
		"Agent",
		firstID,
		"",
		"Updated",
		"Finished",
		nil,
		nil,
		-1,
	)
	if dbusError != nil {
		t.Fatal(dbusError)
	}
	if secondID != firstID {
		t.Fatalf("replacement id = %d, want %d", secondID, firstID)
	}
	page, err := queue.page("")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Notifications) != 2 ||
		page.Notifications[0].Urgency != "critical" ||
		page.Notifications[1].Title != "Updated" {
		t.Fatalf("notifications = %#v", page.Notifications)
	}
}

func TestNotificationServerBoundsText(t *testing.T) {
	queue, err := newNotificationQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	server := newNotificationServer(nil, queue)
	_, _ = server.Notify(
		strings.Repeat("a", maxApplicationRunes+1),
		0,
		"",
		strings.Repeat("t", maxTitleRunes+1),
		strings.Repeat("b", maxBodyRunes+1),
		nil,
		nil,
		-1,
	)
	page, err := queue.page("")
	if err != nil {
		t.Fatal(err)
	}
	item := page.Notifications[0]
	if len([]rune(item.App)) != maxApplicationRunes ||
		len([]rune(item.Title)) != maxTitleRunes ||
		len([]rune(item.Body)) != maxBodyRunes {
		t.Fatalf("notification was not bounded: %#v", item)
	}
}

func TestNotificationServerBoundsActiveState(t *testing.T) {
	queue, err := newNotificationQueue(maxActiveItems + 1)
	if err != nil {
		t.Fatal(err)
	}
	server := newNotificationServer(nil, queue)
	for index := 0; index < maxActiveItems+1; index++ {
		_, dbusError := server.Notify(
			"Agent",
			0,
			"",
			"Event",
			"",
			nil,
			nil,
			-1,
		)
		if dbusError != nil {
			t.Fatal(dbusError)
		}
	}
	if len(server.active) != maxActiveItems ||
		len(server.activeOrder) != maxActiveItems {
		t.Fatalf(
			"active state = %d map entries, %d ordered entries",
			len(server.active),
			len(server.activeOrder),
		)
	}
	if _, exists := server.active[1]; exists {
		t.Fatal("oldest notification remained active")
	}
}
