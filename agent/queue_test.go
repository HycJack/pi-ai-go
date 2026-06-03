package agent

import (
	"testing"
	"time"

	core "pi-ai-go/core"
)

func TestMessageQueueDrain(t *testing.T) {
	q := NewMessageQueue()
	q.Steer(core.UserMessage{Content: "steer 1", Timestamp: time.Now()})
	q.FollowUp(core.UserMessage{Content: "follow 1", Timestamp: time.Now()})
	if q.Len() != 2 {
		t.Errorf("Len = %d", q.Len())
	}
	s := q.DrainSteering()
	if len(s) != 1 {
		t.Errorf("steer count = %d", len(s))
	}
	if q.DrainSteering() != nil {
		t.Error("steer should be empty")
	}
	f := q.DrainFollowUp()
	if len(f) != 1 {
		t.Errorf("follow count = %d", len(f))
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d", q.Len())
	}
}

func TestMessageQueueDrainAll(t *testing.T) {
	q := NewMessageQueue()
	q.Steer(core.UserMessage{Content: "s", Timestamp: time.Now()})
	q.FollowUp(core.UserMessage{Content: "f", Timestamp: time.Now()})
	s, f := q.DrainAll()
	if len(s) != 1 || len(f) != 1 {
		t.Errorf("drain all: s=%d f=%d", len(s), len(f))
	}
	if q.Len() != 0 {
		t.Error("queue should be empty")
	}
}

func TestMessageQueueMode(t *testing.T) {
	q := NewMessageQueue()
	if q.Mode() != QueueModeSteer {
		t.Errorf("default mode = %s", q.Mode())
	}
	q.SetMode(QueueModeInterrupt)
	if q.Mode() != QueueModeInterrupt {
		t.Error("mode not updated")
	}
}
