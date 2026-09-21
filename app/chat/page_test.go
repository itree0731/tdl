package chat

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestNextDialogCursorUsesLastDialogAndMessageDate(t *testing.T) {
	entities := pageEntities([]tg.UserClass{&tg.User{ID: 7, AccessHash: 70}}, nil)
	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 7}, TopMessage: 10},
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 7}, TopMessage: 11},
	}
	messages := map[int]tg.NotEmptyMessage{11: &tg.Message{ID: 11, Date: 1234}}
	cursor, err := nextDialogCursor(dialogs, messages, entities, 2)
	if err != nil {
		t.Fatal(err)
	}
	if cursor == nil || cursor.OffsetID != 11 || cursor.OffsetDate != 1234 {
		t.Fatalf("cursor = %+v", cursor)
	}
	peer, ok := cursor.OffsetPeer.(*tg.InputPeerUser)
	if !ok || peer.UserID != 7 || peer.AccessHash != 70 {
		t.Fatalf("offset peer = %#v", cursor.OffsetPeer)
	}
}

func TestNextDialogCursorEndsOnShortPage(t *testing.T) {
	entities := pageEntities([]tg.UserClass{&tg.User{ID: 7, AccessHash: 70}}, nil)
	dialogs := []tg.DialogClass{&tg.Dialog{Peer: &tg.PeerUser{UserID: 7}, TopMessage: 10}}
	cursor, err := nextDialogCursor(dialogs, nil, entities, 100)
	if err != nil || cursor != nil {
		t.Fatalf("cursor=%+v err=%v", cursor, err)
	}
}

func TestUnpackDialogsPagePreservesOnePage(t *testing.T) {
	response := &tg.MessagesDialogsSlice{
		Count:   1,
		Dialogs: []tg.DialogClass{&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}}},
		Users:   []tg.UserClass{&tg.User{ID: 1}},
	}
	dialogs, _, users, _, err := unpackDialogsPage(response)
	if err != nil || len(dialogs) != 1 || len(users) != 1 {
		t.Fatalf("dialogs=%d users=%d err=%v", len(dialogs), len(users), err)
	}
}
