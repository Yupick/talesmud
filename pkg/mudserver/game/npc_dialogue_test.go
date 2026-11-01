package game

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/dialogs"
)

func TestMatchAuthoredDialogLabelFindsNodeAndOption(t *testing.T) {
	root := &dialogs.Dialog{
		Entity: &entities.Entity{ID: "dialog-1"},
		NodeID: "main",
		Text:   "Bienvenido, viajero.",
		Options: []*dialogs.Dialog{
			{
				Entity: &entities.Entity{ID: "option-1"},
				NodeID: "walls",
				Text:   "Preguntar por los muros",
			},
			{
				Entity: &entities.Entity{ID: "option-2"},
				NodeID: "bye",
				Text:   "Despedirse",
			},
		},
	}

	if got := matchAuthoredDialogLabel(root, "¿Preguntar por los muros?"); got != "walls" {
		t.Fatalf("option label matched %q, want walls", got)
	}
	if got := matchAuthoredDialogLabel(root, "  bienvenido,   viajero "); got != "main" {
		t.Fatalf("node text matched %q, want main", got)
	}
	if got := matchAuthoredDialogLabel(root, "something else"); got != "" {
		t.Fatalf("unexpected match %q", got)
	}
}
