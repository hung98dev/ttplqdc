package id

import "testing"

func TestServerJobIdDeterministic(t *testing.T) {
	target := NewV4().String()
	window := "1759363200000"
	a := ServerJobOperationID("job.auction", "AUCTION_EXPIRY", target, window)
	b := ServerJobOperationID("job.auction", "AUCTION_EXPIRY", target, window)
	if a != b {
		t.Fatal("server job operation ID is not deterministic across retries")
	}
	if a.IsNil() {
		t.Fatal("valid job key produced the nil UUID")
	}
	want := V5(ServerJobNamespaceUUID, "job.auction:AUCTION_EXPIRY:"+target+":"+window)
	if a != want {
		t.Fatal("job operation ID is not UUIDv5(SERVER_JOB_NAMESPACE_UUID, family:job_key)")
	}
	if c := ServerJobOperationID("job.auction", "AUCTION_FALLBACK", target, window); c == a {
		t.Fatal("different job key produced the same operation ID")
	}
	if c := ServerJobOperationID("job.maintenance", "AUCTION_EXPIRY", target, window); c == a {
		t.Fatal("different family produced the same operation ID")
	}

	// ':' inside a component or an empty component/family is rejected with
	// the nil (malformed) UUID; a job_key is always required.
	invalid := [][]string{
		{"KIND:BAD", target},
		{"", target},
		{target, ""},
	}
	for _, comps := range invalid {
		if got := ServerJobOperationID("job.auction", comps...); !got.IsNil() {
			t.Fatalf("component %v accepted", comps)
		}
	}
	if got := ServerJobOperationID("", "KIND"); !got.IsNil() {
		t.Fatal("empty family accepted")
	}
	if got := ServerJobOperationID("job.auction"); !got.IsNil() {
		t.Fatal("missing job_key accepted")
	}

	cursor := MaintenanceCursorKey([]byte{0x00, 0xAB, 0xcd})
	if cursor != "00abcd" {
		t.Fatalf("MaintenanceCursorKey = %q, want lowercase hex", cursor)
	}
}

func TestServerJobNamespacePinned(t *testing.T) {
	if got := ServerJobNamespaceUUID.String(); got != "64d34c40-8657-462b-887f-5970db9eaa5f" {
		t.Fatalf("SERVER_JOB_NAMESPACE_UUID = %q, want the pinned 64d34c40-8657-462b-887f-5970db9eaa5f", got)
	}
	if got := ContentGrantNamespaceUUID.String(); got != "f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b" {
		t.Fatalf("CONTENT_GRANT_NAMESPACE_UUID = %q, want the pinned f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b", got)
	}
}
