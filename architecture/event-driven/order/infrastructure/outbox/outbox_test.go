package outbox

import (
	"context"
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

// OutboxRelayTest 的 Go 对照（fake repo + fake deliver 对应 Mockito），
// 外加一条 Go 增强用例：投递失败不标记 → 下轮重投（at-least-once 的核心）。

// fakeRepo 内存版 OutboxRepository（对应 @Mock OutboxJpaRepository）。
type fakeRepo struct {
	pending   []*OutboxMessageEntity
	published map[int64]bool
}

func newFakeRepo(msgs ...*OutboxMessageEntity) *fakeRepo {
	return &fakeRepo{pending: msgs, published: map[int64]bool{}}
}

func (r *fakeRepo) Save(_ context.Context, msg *OutboxMessageEntity) error {
	r.pending = append(r.pending, msg)
	return nil
}

func (r *fakeRepo) FindPending(_ context.Context, limit int) ([]*OutboxMessageEntity, error) {
	if len(r.pending) > limit {
		return r.pending[:limit], nil
	}
	return r.pending, nil
}

func (r *fakeRepo) MarkPublished(_ context.Context, id int64) error {
	r.published[id] = true
	return nil
}

// fakeDeliver 记录投递调用（对应 @Mock KafkaTemplate.verify）。
type fakeDeliver struct {
	calls   []event.Envelope
	failIDs map[string]bool // 这些 eventId 投递失败
}

func newFakeDeliver(failIDs ...string) *fakeDeliver {
	set := map[string]bool{}
	for _, id := range failIDs {
		set[id] = true
	}
	return &fakeDeliver{failIDs: set}
}

func (f *fakeDeliver) Deliver(_ context.Context, envelope event.Envelope) error {
	if f.failIDs[envelope.EventID] {
		return errDeliveryFailed
	}
	f.calls = append(f.calls, envelope)
	return nil
}

var errDeliveryFailed = &deliveryError{}

type deliveryError struct{}

func (*deliveryError) Error() string { return "delivery failed" }

func sampleMessage(eventID string) *OutboxMessageEntity {
	return &OutboxMessageEntity{
		ID:          1,
		EventID:     eventID,
		Topic:       event.TopicOrderEvents,
		AggregateID: "order-1",
		EventType:   event.TypeOrderCreated,
		Payload:     `{"eventId":"` + eventID + `","type":"OrderCreatedEvent","aggregateId":"order-1","topic":"order-events","payload":{}}`,
	}
}

// 对应 Java：relay_publishes_pending_messages_and_marks_published。
func TestRelayPublishesPendingMessagesAndMarksPublished(t *testing.T) {
	msg := sampleMessage("event-1")
	repo := newFakeRepo(msg)
	deliver := newFakeDeliver()

	relay := NewRelay(repo, deliver.Deliver, 0)
	if err := relay.RelayOnce(context.Background()); err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}

	if len(deliver.calls) != 1 {
		t.Fatalf("deliver calls = %d, want 1", len(deliver.calls))
	}
	if deliver.calls[0].EventID != "event-1" || deliver.calls[0].Topic != event.TopicOrderEvents {
		t.Fatalf("delivered wrong envelope: %+v", deliver.calls[0])
	}
	if !repo.published[msg.ID] {
		t.Fatal("message must be marked published after successful delivery")
	}
}

// 对应 Java：relay_with_empty_outbox_does_nothing。
func TestRelayWithEmptyOutboxDoesNothing(t *testing.T) {
	repo := newFakeRepo()
	deliver := newFakeDeliver()

	relay := NewRelay(repo, deliver.Deliver, 0)
	if err := relay.RelayOnce(context.Background()); err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}

	if len(deliver.calls) != 0 {
		t.Fatalf("deliver must not be called, got %d calls", len(deliver.calls))
	}
}

// Go 增强：投递失败不标记 → 留在 outbox 下轮重投（at-least-once 语义）。
func TestRelayKeepsFailedMessagesPending(t *testing.T) {
	ok := sampleMessage("event-ok")
	bad := sampleMessage("event-bad")
	bad.ID = 2
	repo := newFakeRepo(ok, bad)
	deliver := newFakeDeliver("event-bad")

	relay := NewRelay(repo, deliver.Deliver, 0)
	if err := relay.RelayOnce(context.Background()); err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}

	if !repo.published[ok.ID] {
		t.Fatal("successful message must be marked published")
	}
	if repo.published[bad.ID] {
		t.Fatal("failed message must stay pending for redelivery")
	}
	if len(deliver.calls) != 1 {
		t.Fatalf("only the ok message should have been delivered, got %d", len(deliver.calls))
	}
}
