package validate

import (
	"testing"

	"nodeforge/internal/model"
)

func TestNodeValidation(t *testing.T) {
	t.Parallel()

	node := model.Node{
		Name:    "broken",
		Type:    model.ProtocolVMess,
		Server:  "",
		Port:    70000,
		Network: "ws",
	}

	errs := Node(node)
	if len(errs) < 3 {
		t.Fatalf("expected multiple validation errors, got %d", len(errs))
	}
}
