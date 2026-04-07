package embed

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestVocab creates a minimal BERT vocab.txt for testing.
func writeTestVocab(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "vocab.txt")

	// Minimal vocab with special tokens and a few words/subwords.
	vocab := "[PAD]\n[UNK]\n[CLS]\n[SEP]\n[MASK]\nhello\nworld\ntest\n##ing\nthe\na\n.\n,\n!"
	if err := os.WriteFile(path, []byte(vocab), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBertTokenizer_Encode(t *testing.T) {
	vocabPath := writeTestVocab(t, t.TempDir())
	tok, err := NewBertTokenizer(vocabPath, 512)
	if err != nil {
		t.Fatalf("NewBertTokenizer: %v", err)
	}

	out := tok.Encode("Hello world!")
	// Expected: [CLS]=2, hello=5, world=6, !=13, [SEP]=3
	if len(out.InputIDs) != 5 {
		t.Fatalf("expected 5 tokens, got %d: %v", len(out.InputIDs), out.InputIDs)
	}
	if out.InputIDs[0] != 2 { // [CLS]
		t.Errorf("first token should be [CLS] (2), got %d", out.InputIDs[0])
	}
	if out.InputIDs[len(out.InputIDs)-1] != 3 { // [SEP]
		t.Errorf("last token should be [SEP] (3), got %d", out.InputIDs[len(out.InputIDs)-1])
	}

	// Attention mask should be all 1s.
	for i, m := range out.AttentionMask {
		if m != int64(1) {
			t.Errorf("AttentionMask[%d] = %d, want 1", i, m)
		}
	}
}

func TestBertTokenizer_WordPiece(t *testing.T) {
	vocabPath := writeTestVocab(t, t.TempDir())
	tok, err := NewBertTokenizer(vocabPath, 512)
	if err != nil {
		t.Fatalf("NewBertTokenizer: %v", err)
	}

	// "testing" should be split into "test" + "##ing"
	out := tok.Encode("testing")
	// [CLS]=2, test=7, ##ing=8, [SEP]=3
	if len(out.InputIDs) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(out.InputIDs), out.InputIDs)
	}
	if out.InputIDs[1] != 7 { // "test"
		t.Errorf("token[1] should be 'test' (7), got %d", out.InputIDs[1])
	}
	if out.InputIDs[2] != 8 { // "##ing"
		t.Errorf("token[2] should be '##ing' (8), got %d", out.InputIDs[2])
	}
}

func TestBertTokenizer_UnknownWord(t *testing.T) {
	vocabPath := writeTestVocab(t, t.TempDir())
	tok, err := NewBertTokenizer(vocabPath, 512)
	if err != nil {
		t.Fatalf("NewBertTokenizer: %v", err)
	}

	// "xyz" is not in vocab and has no subword matches.
	out := tok.Encode("xyz")
	// [CLS]=2, [UNK]=1, [SEP]=3
	if len(out.InputIDs) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(out.InputIDs), out.InputIDs)
	}
	if out.InputIDs[1] != 1 { // [UNK]
		t.Errorf("unknown word should map to [UNK] (1), got %d", out.InputIDs[1])
	}
}

func TestBertTokenizer_Truncation(t *testing.T) {
	vocabPath := writeTestVocab(t, t.TempDir())
	tok, err := NewBertTokenizer(vocabPath, 5) // maxLen = 5
	if err != nil {
		t.Fatalf("NewBertTokenizer: %v", err)
	}

	// "hello world test" = 3 content tokens, but maxLen=5 means max 3 content + 2 special.
	out := tok.Encode("hello world test")
	if len(out.InputIDs) != 5 {
		t.Fatalf("expected 5 tokens (truncated), got %d: %v", len(out.InputIDs), out.InputIDs)
	}
}

func TestBertTokenizer_EmptyInput(t *testing.T) {
	vocabPath := writeTestVocab(t, t.TempDir())
	tok, err := NewBertTokenizer(vocabPath, 512)
	if err != nil {
		t.Fatalf("NewBertTokenizer: %v", err)
	}

	out := tok.Encode("")
	// Should just be [CLS] [SEP].
	if len(out.InputIDs) != 2 {
		t.Fatalf("expected 2 tokens for empty input, got %d: %v", len(out.InputIDs), out.InputIDs)
	}
}

func TestBertTokenizer_Punctuation(t *testing.T) {
	vocabPath := writeTestVocab(t, t.TempDir())
	tok, err := NewBertTokenizer(vocabPath, 512)
	if err != nil {
		t.Fatalf("NewBertTokenizer: %v", err)
	}

	out := tok.Encode("hello, world.")
	// "hello" "," "world" "." → 4 content tokens + 2 special = 6
	if len(out.InputIDs) != 6 {
		t.Fatalf("expected 6 tokens, got %d: %v", len(out.InputIDs), out.InputIDs)
	}
}

func TestStripAccents(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"cafe", "cafe"},
		{"caf\u00e9", "cafe"},   // é → e
		{"na\u00efve", "naive"}, // ï → i
		{"r\u00e9sum\u00e9", "resume"},
	}
	for _, tc := range tests {
		got := stripAccents(tc.in)
		if got != tc.want {
			t.Errorf("stripAccents(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
