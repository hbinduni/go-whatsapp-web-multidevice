package user

import (
	"encoding/json"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestBlocklistResponsePreservesJSONContract(t *testing.T) {
	response := BlocklistResponse{Data: &types.Blocklist{
		DHash: "revision",
		Items: []types.BlocklistItem{
			{LID: types.NewJID("1001", types.HiddenUserServer), PN: types.NewJID("15550000001", types.DefaultUserServer), Active: true},
			{LID: types.NewJID("1002", types.HiddenUserServer), Active: true},
			{LID: types.NewJID("1003", types.HiddenUserServer), PN: types.NewJID("15550000003", types.DefaultUserServer), Active: false},
		},
	}}
	got, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"DHash":"revision","JIDs":["15550000001@s.whatsapp.net","1002@lid","15550000003@s.whatsapp.net"]}}`
	if string(got) != want {
		t.Fatalf("blocklist JSON = %s, want %s", got, want)
	}
}

func TestBlocklistResponseEmptyAndNil(t *testing.T) {
	for _, tc := range []struct {
		name string
		data *types.Blocklist
		want string
	}{
		{name: "nil", want: `{"data":null}`},
		{name: "empty", data: &types.Blocklist{DHash: "empty"}, want: `{"data":{"DHash":"empty","JIDs":null}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(BlocklistResponse{Data: tc.data})
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("blocklist JSON = %s, want %s", got, tc.want)
			}
		})
	}
}
