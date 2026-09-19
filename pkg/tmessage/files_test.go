package tmessage

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/stretchr/testify/assert"
)

func testPeer() peers.Peer {
	m := (&peers.Options{}).Build(nil)
	return m.User(&tg.User{ID: 123456, FirstName: "test"})
}

// genExport builds a Telegram export-like JSON with n plain messages.
func genExport(n int) []byte {
	var b bytes.Buffer
	b.WriteString(`{"id": 123456, "name": "test", "type": "personal_chat", "messages": [`)
	for i := 1; i <= n; i++ {
		if i > 1 {
			b.WriteByte(',')
		}
		b.WriteString(`{"id": ` + strconv.Itoa(i) +
			`, "type": "message", "date_unixtime": "1700000000", "text": "hello"}`)
	}
	b.WriteString(`]}`)
	return b.Bytes()
}

func TestCollectAllMessages(t *testing.T) {
	// regression: exports must parse completely. The old implementation ran
	// two jstream decoders on the same *os.File (via Seek(0)), which raced
	// on the shared file offset and silently lost almost all messages.
	const n = 20000
	d, err := collect(context.Background(), bytes.NewReader(genExport(n)), testPeer(), false)
	assert.NoError(t, err)
	assert.Len(t, d.Messages, n)
}

func TestCollectOnlyMediaFilters(t *testing.T) {
	d, err := collect(context.Background(), bytes.NewReader(genExport(100)), testPeer(), true)
	assert.NoError(t, err)
	assert.Empty(t, d.Messages) // no message has file/photo
}

func TestCollectTruncatedExport(t *testing.T) {
	// regression: a truncated export must be reported as an error.
	// jstream ends the stream silently, only d.Err() carries the failure.
	src := genExport(100)
	truncated := src[:len(src)-200]

	_, err := collect(context.Background(), bytes.NewReader(truncated), testPeer(), false)
	assert.Error(t, err)
}
