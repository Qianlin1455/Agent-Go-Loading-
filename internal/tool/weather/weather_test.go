package weather

import (
	"context"
	"testing"
)

func TestExecute(t *testing.T) {
	result, err := New().Execute(context.Background(), `{"location":"杭州"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "杭州：25℃，晴天" {
		t.Fatalf("unexpected result: %s", result)
	}
}
