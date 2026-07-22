package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestPlaybookRevisionsAreImmutableAndLatestIsListed(t *testing.T) {
	s := openTemp(t)

	r1, err := s.SavePlaybookRevision("feature-work", `{"name":"Feature work","value":1}`)
	if err != nil {
		t.Fatalf("save r1: %v", err)
	}
	r2, err := s.SavePlaybookRevision("feature-work", `{"name":"Feature work","value":2}`)
	if err != nil {
		t.Fatalf("save r2: %v", err)
	}
	if r1.Revision != 1 || r2.Revision != 2 {
		t.Fatalf("revisions = %d,%d want 1,2", r1.Revision, r2.Revision)
	}

	old, err := s.PlaybookRevision("feature-work", 1)
	if err != nil || old.Data != r1.Data {
		t.Fatalf("old revision changed: %+v err=%v", old, err)
	}
	latest, err := s.LatestPlaybookRevisions()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if len(latest) != 1 || latest[0].Revision != 2 || latest[0].Data != r2.Data {
		t.Fatalf("latest = %+v", latest)
	}
}

func TestPlaybookRevisionLookupRequiresExactRevision(t *testing.T) {
	s := openTemp(t)
	if _, err := s.SavePlaybookRevision("quick-answer", `{"name":"Quick answer"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlaybookRevision("quick-answer", 2); err == nil {
		t.Fatal("missing immutable revision should error")
	}
	if _, err := s.PlaybookRevision("missing", 1); err == nil {
		t.Fatal("missing playbook should error")
	}
}

func TestSeedPlaybookRevisionsIsAtomicAndOnlySeedsAnEmptyStore(t *testing.T) {
	s := openTemp(t)
	duplicate := []PlaybookRevision{
		{ID: "feature-work", Revision: 1, Data: `{"name":"Feature work"}`},
		{ID: "feature-work", Revision: 1, Data: `{"name":"duplicate"}`},
	}
	if err := s.SeedPlaybookRevisions(duplicate); err == nil {
		t.Fatal("duplicate seed unexpectedly succeeded")
	}
	latest, err := s.LatestPlaybookRevisions()
	if err != nil || len(latest) != 0 {
		t.Fatalf("failed seed was not atomic: latest=%+v err=%v", latest, err)
	}

	seed := []PlaybookRevision{
		{ID: "feature-work", Revision: 1, Data: `{"name":"Feature work"}`},
		{ID: "quick-answer", Revision: 1, Data: `{"name":"Quick answer"}`},
	}
	if err := s.SeedPlaybookRevisions(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.SeedPlaybookRevisions([]PlaybookRevision{{ID: "later", Revision: 1, Data: `{}`}}); err != nil {
		t.Fatalf("idempotent seed: %v", err)
	}
	latest, err = s.LatestPlaybookRevisions()
	if err != nil || len(latest) != 2 {
		t.Fatalf("seeded latest=%+v err=%v", latest, err)
	}
}

func TestSavePlaybookRevisionIfLatestRejectsStaleExpectedRevision(t *testing.T) {
	s := openTemp(t)
	first, err := s.SavePlaybookRevisionIfLatest("feature-work", 0, `{"name":"Feature work"}`)
	if err != nil || first.Revision != 1 {
		t.Fatalf("first conditional save = %+v err=%v", first, err)
	}
	if _, err := s.SavePlaybookRevisionIfLatest("feature-work", 0, `{"name":"stale"}`); !errors.Is(err, ErrPlaybookRevisionStale) {
		t.Fatalf("stale conditional save error = %v, want ErrPlaybookRevisionStale", err)
	}
	latest, err := s.LatestPlaybookRevisions()
	if err != nil || len(latest) != 1 || latest[0].Revision != 1 || latest[0].Data != first.Data {
		t.Fatalf("latest after stale save = %+v err=%v", latest, err)
	}
}

func TestSavePlaybookRevisionIfLatestIdentifiesStaleAcrossStoreHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fort.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close() })
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { second.Close() })

	if _, err := first.SavePlaybookRevisionIfLatest("feature-work", 0, `{"name":"first"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := second.SavePlaybookRevisionIfLatest("feature-work", 0, `{"name":"stale"}`); !errors.Is(err, ErrPlaybookRevisionStale) {
		t.Fatalf("cross-store stale error = %v, want ErrPlaybookRevisionStale", err)
	}
}
