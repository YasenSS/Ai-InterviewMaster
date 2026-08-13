package retriever

import "testing"

func TestPrivateSearchRequiresUserID(t *testing.T) {
	r := PGVector{}
	_, err := r.Search(t.Context(), Query{Corpus: CorpusPrivate, Text: "kafka"})
	if err == nil {
		t.Fatal("private search without user_id succeeded")
	}
}

func TestUnknownCorpusRejected(t *testing.T) {
	r := PGVector{BindUser: "user-1"}
	_, err := r.Search(t.Context(), Query{Corpus: "other_users", Text: "x", UserID: "user-1"})
	if err == nil {
		t.Fatal("unknown corpus accepted")
	}
}
