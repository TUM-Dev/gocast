package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// The old UI reads these frames by key (web/ts/components/chat.ts), so the mirror has
// to keep v1's exact shape.
func TestPollMirrorFramesKeepV1sShape(t *testing.T) {
	poll := model.Poll{
		Question: "Q?",
		PollOptions: []model.PollOption{
			{Model: gorm.Model{ID: 601}, Answer: "yes"},
			{Model: gorm.Model{ID: 602}, Answer: "no"},
		},
	}
	decode := func(v any) map[string]any {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return out
	}

	started := decode(pollStartedPayload(poll))
	want := map[string]any{
		"type": "start_poll", "active": true, "question": "Q?", "submitted": float64(0),
		"options": []any{
			map[string]any{"ID": float64(601), "answer": "yes", "votes": float64(0)},
			map[string]any{"ID": float64(602), "answer": "no", "votes": float64(0)},
		},
	}
	if !reflect.DeepEqual(started, want) {
		t.Errorf("start frame = %v, want %v", started, want)
	}

	closed := decode(pollClosedPayload(poll, map[uint]int64{601: 5, 602: 2}))
	want = map[string]any{
		"type": "close_active_poll", "question": "Q?",
		"options": []any{
			map[string]any{"ID": float64(601), "answer": "yes", "votes": float64(5)},
			map[string]any{"ID": float64(602), "answer": "no", "votes": float64(2)},
		},
	}
	if !reflect.DeepEqual(closed, want) {
		t.Errorf("close frame = %v, want %v", closed, want)
	}
}
