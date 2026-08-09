package transporttrust

import (
	"context"
	"testing"
)

func TestTrustedIsInProcessContextOnly(t *testing.T) {
	if Trusted(context.Background()) {
		t.Fatal("unmarked context is trusted")
	}
	if !Trusted(WithTrusted(context.Background())) {
		t.Fatal("marked context is not trusted")
	}
}
