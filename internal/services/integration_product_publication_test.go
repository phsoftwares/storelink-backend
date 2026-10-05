package services

import "testing"

func TestPublicationVersionIsSafeForLegacyJSONClients(t *testing.T) {
	value := publicationVersion("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	const maxExactJSONInteger int64 = 9007199254740991
	if value <= 0 || value > maxExactJSONInteger {
		t.Fatalf("publication version %d is outside the exact JSON number range", value)
	}
	if value != publicationVersion("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff") {
		t.Fatal("publication version must be deterministic")
	}
}

func TestNextLocalCodeCandidateSkipsWithFixedWidth(t *testing.T) {
	if got := nextLocalCodeCandidate("005782"); got != "005783" {
		t.Fatalf("candidate after 005782 = %q", got)
	}
	if got := nextLocalCodeCandidate("999999"); got != "" {
		t.Fatalf("candidate after upper bound = %q", got)
	}
	if got := nextLocalCodeCandidate("ABC"); got != "" {
		t.Fatalf("candidate after nonnumeric code = %q", got)
	}
}
