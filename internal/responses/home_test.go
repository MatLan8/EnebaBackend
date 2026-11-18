package responses

import (
	"encoding/json"
	"testing"
)

func TestNewHomeResponseSkipsEmptyBlocks(t *testing.T) {
	blocks := []HomeBlockResponse{
		{Title: "Games", Products: []ProductListItemResponse{{Title: "Elden Ring"}}},
		{Title: "Subscriptions", Products: []ProductListItemResponse{}},
		{Title: "Add-ons", Products: nil},
		{Title: "Software", Products: []ProductListItemResponse{{Title: "Windows 11"}}},
	}

	got := NewHomeResponse(blocks)

	if len(got.Blocks) != 2 || got.Blocks[0].Title != "Games" || got.Blocks[1].Title != "Software" {
		t.Errorf("blocks = %+v; want Games and Software, in that order", got.Blocks)
	}
}

func TestNewHomeResponseNoBlocksIsEmptyJSONArray(t *testing.T) {
	got := NewHomeResponse([]HomeBlockResponse{{Title: "Games"}})

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"blocks":[]}` {
		t.Errorf("JSON = %s; want {\"blocks\":[]}", body)
	}
}
