package capability

import (
	"context"
	"fmt"
	"strings"

	"github.com/tobsai/fort/exec/codexsubscription"
)

type verifiedCodexCommands interface {
	Hold(string) (*HeldCommand, error)
	ResolveVerifiedExecutable(string) (string, error)
}

// CodexSubscriptionResolver bridges capability-authorized, content-addressed
// Codex bytes into the isolated subscription runtime. It returns no account
// identifiers or auth paths beyond the process-private environment required by
// the already authenticated Codex executable.
type CodexSubscriptionResolver struct {
	commands verifiedCodexCommands
}

func NewCodexSubscriptionResolver(resolver *CommandResolver) *CodexSubscriptionResolver {
	return newCodexSubscriptionResolver(resolver)
}

func newCodexSubscriptionResolver(commands verifiedCodexCommands) *CodexSubscriptionResolver {
	return &CodexSubscriptionResolver{commands: commands}
}

func (r *CodexSubscriptionResolver) ResolveCodex(ctx context.Context) (codexsubscription.HeldExecutable, error) {
	if err := ctx.Err(); err != nil {
		return codexsubscription.HeldExecutable{}, err
	}
	if r == nil || r.commands == nil {
		return codexsubscription.HeldExecutable{}, fmt.Errorf("capability command: Codex subscription authority unavailable")
	}
	held, err := r.commands.Hold("codex")
	if err != nil || held == nil || !codexsubscription.AcceptsCodexExecutableRevision(held.Digest()) {
		return codexsubscription.HeldExecutable{}, fmt.Errorf("capability command: Codex subscription authority unavailable")
	}
	verifiedPath, err := r.commands.ResolveVerifiedExecutable("codex")
	if err != nil || verifiedPath == "" || verifiedPath != held.Executable() {
		return codexsubscription.HeldExecutable{}, fmt.Errorf("capability command: Codex subscription authority unavailable")
	}
	return codexsubscription.HeldExecutable{
		Path: held.Executable(), Version: codexsubscription.CodexVersion,
		ExecutableRevision: held.Digest(), SchemaRevision: codexsubscription.CodexSchemaRevision,
		Environment: subscriptionEnvironment(held.Environment()),
	}, nil
}

func subscriptionEnvironment(environment []string) []string {
	allowed := map[string]bool{
		"HOME": true, "CODEX_HOME": true, "PATH": true, "TMPDIR": true,
		"LANG": true, "LC_ALL": true, "SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	}
	out := make([]string, 0, len(allowed))
	seen := map[string]bool{}
	for _, entry := range environment {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || !allowed[key] || seen[key] || len(entry) > 4096 {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out
}
