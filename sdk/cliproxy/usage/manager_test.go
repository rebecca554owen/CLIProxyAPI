package usage

import (
	"context"
	"testing"
)

type blockingPlugin struct {
	started chan struct{}
	release chan struct{}
}

func (p blockingPlugin) HandleUsage(context.Context, Record) {
	p.started <- struct{}{}
	<-p.release
}

func TestManagerPublishBoundsQueue(t *testing.T) {
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	manager := NewManager(2)
	manager.Register(blockingPlugin{started: started, release: release})
	manager.Start(context.Background())

	manager.Publish(context.Background(), Record{Model: "first"})
	<-started

	manager.Publish(context.Background(), Record{Model: "second"})
	manager.Publish(context.Background(), Record{Model: "third"})
	manager.Publish(context.Background(), Record{Model: "fourth"})

	manager.mu.Lock()
	if len(manager.queue) != 2 {
		t.Fatalf("queue len = %d, want 2", len(manager.queue))
	}
	if got := manager.queue[0].record.Model; got != "third" {
		t.Fatalf("first queued model = %q, want third", got)
	}
	if got := manager.queue[1].record.Model; got != "fourth" {
		t.Fatalf("second queued model = %q, want fourth", got)
	}
	manager.mu.Unlock()

	if got := manager.Dropped(); got != 1 {
		t.Fatalf("dropped = %d, want 1", got)
	}

	close(release)
	manager.Stop()
}
