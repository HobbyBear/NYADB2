package booter_test

import (
	"NYADB2/backend/utils/booter"
	"bytes"
	"testing"
)

func TestBooter(t *testing.T) {
	bt := booter.Create("/tmp/booter_test")
	raw := []byte("123456jksadhfjksadflkwejflk;n")

	bt.Update(raw)

	if bytes.Compare(raw, bt.Load()) != 0 {
		t.Fatal(raw, " ", bt.Load())
	}
}
