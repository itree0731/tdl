package progress

import (
	"context"
	"testing"
	"time"
)

func TestEventCarriesStructuredTransferState(t *testing.T) {
	seen := make([]Event, 0, 2)
	ctx := WithSink(context.Background(), SinkFunc(func(event Event) { seen = append(seen, event) }))
	Emit(ctx, Event{Kind: KindStarted, Direction: DirectionDownload, Status: StatusRunning, TaskID: "a", TotalBytes: 100, At: time.Unix(1, 0)})
	Emit(ctx, Event{Kind: KindUpdated, Direction: DirectionDownload, Status: StatusRunning, TaskID: "a", CompletedBytes: 25, TotalBytes: 100, At: time.Unix(2, 0)})
	if len(seen) != 2 || seen[1].CompletedBytes != 25 || seen[1].TotalBytes != 100 {
		t.Fatalf("events = %+v", seen)
	}
}
