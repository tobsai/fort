package capability

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/tobsai/fort/exec/codexsubscription"
)

type fakeVerifiedCodexCommands struct {
	held         *HeldCommand
	holdErr      error
	verifiedPath string
	verifiedErr  error
}

func (f fakeVerifiedCodexCommands) Hold(string) (*HeldCommand, error) {
	return f.held, f.holdErr
}

func (f fakeVerifiedCodexCommands) ResolveVerifiedExecutable(string) (string, error) {
	return f.verifiedPath, f.verifiedErr
}

func TestCodexSubscriptionResolverReturnsOnlyAuthorizedCatalogBytes(t *testing.T) {
	held := &HeldCommand{
		path: "/private/staged/codex", digest: codexsubscription.CodexExecutableRevision,
		env: []string{"CODEX_HOME=/private/auth", "PATH=/usr/bin", "OPENAI_API_KEY=PRIVATE-API-KEY", "UNRELATED_SECRET=PRIVATE"},
	}
	resolver := newCodexSubscriptionResolver(fakeVerifiedCodexCommands{held: held, verifiedPath: held.path})
	got, err := resolver.ResolveCodex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != held.path || got.Version != codexsubscription.CodexVersion ||
		got.ExecutableRevision != codexsubscription.CodexExecutableRevision ||
		got.SchemaRevision != codexsubscription.CodexSchemaRevision ||
		!reflect.DeepEqual(got.Environment, []string{"CODEX_HOME=/private/auth", "PATH=/usr/bin"}) {
		t.Fatalf("resolved = %#v", got)
	}
	held.env[0] = "PRIVATE=MUTATED"
	if got.Environment[0] == held.env[0] {
		t.Fatal("resolver returned mutable held environment")
	}
}

func TestCodexSubscriptionResolverFailsClosedOnIdentityOrAuthorizationDrift(t *testing.T) {
	valid := &HeldCommand{path: "/private/staged/codex", digest: codexsubscription.CodexExecutableRevision}
	tests := []fakeVerifiedCodexCommands{
		{holdErr: ErrCommandAbsent},
		{held: &HeldCommand{path: valid.path, digest: "wrong"}, verifiedPath: valid.path},
		{held: valid, verifiedErr: ErrCommandIdentityChanged},
		{held: valid, verifiedPath: "/private/staged/other"},
	}
	for index, commands := range tests {
		resolver := newCodexSubscriptionResolver(commands)
		if _, err := resolver.ResolveCodex(context.Background()); err == nil {
			t.Fatalf("case %d accepted drift", index)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newCodexSubscriptionResolver(fakeVerifiedCodexCommands{held: valid, verifiedPath: valid.path}).ResolveCodex(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
}
