package embed

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// TokenizerOutput holds the encoded inputs for a BERT model.
type TokenizerOutput struct {
	InputIDs      []int64
	AttentionMask []int64
}

// BertTokenizer implements BERT WordPiece tokenization.
type BertTokenizer struct {
	vocab  map[string]int32
	maxLen int
	clsID  int32
	sepID  int32
	unkID  int32
}

// NewBertTokenizer loads a BERT vocabulary from vocab.txt.
func NewBertTokenizer(vocabPath string, maxLen int) (*BertTokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("open vocab: %w", err)
	}
	defer f.Close()

	vocab := make(map[string]int32)
	scanner := bufio.NewScanner(f)
	var id int32
	for scanner.Scan() {
		vocab[scanner.Text()] = id
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read vocab: %w", err)
	}

	return &BertTokenizer{
		vocab:  vocab,
		maxLen: maxLen,
		clsID:  vocab["[CLS]"],
		sepID:  vocab["[SEP]"],
		unkID:  vocab["[UNK]"],
	}, nil
}

// Encode tokenizes text and returns model inputs.
func (t *BertTokenizer) Encode(text string) TokenizerOutput {
	text = normalize(text)
	tokens := t.tokenize(text)

	// Truncate to maxLen - 2 for [CLS] and [SEP].
	if len(tokens) > t.maxLen-2 {
		tokens = tokens[:t.maxLen-2]
	}

	n := len(tokens) + 2
	out := TokenizerOutput{
		InputIDs:      make([]int64, n),
		AttentionMask: make([]int64, n),
	}

	out.InputIDs[0] = int64(t.clsID)
	for i, tok := range tokens {
		out.InputIDs[i+1] = int64(tok)
	}
	out.InputIDs[n-1] = int64(t.sepID)

	for i := range out.AttentionMask {
		out.AttentionMask[i] = 1
	}

	return out
}

func normalize(text string) string {
	text = strings.ToLower(text)
	text = stripAccents(text)
	return cleanText(text)
}

func (t *BertTokenizer) tokenize(text string) []int32 {
	var ids []int32
	for _, word := range strings.Fields(text) {
		for _, sub := range splitOnPunctuation(word) {
			ids = append(ids, t.wordPiece(sub)...)
		}
	}
	return ids
}

const maxWordLen = 200

func (t *BertTokenizer) wordPiece(word string) []int32 {
	runes := []rune(word)
	if len(runes) > maxWordLen {
		return []int32{t.unkID}
	}

	var tokens []int32
	start := 0
	for start < len(runes) {
		end := len(runes)
		var matched bool
		for end > start {
			substr := string(runes[start:end])
			if start > 0 {
				substr = "##" + substr
			}
			if id, ok := t.vocab[substr]; ok {
				tokens = append(tokens, id)
				matched = true
				start = end
				break
			}
			end--
		}
		if !matched {
			return []int32{t.unkID}
		}
	}
	return tokens
}

func stripAccents(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.In(r, unicode.Mn) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cleanText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0 || r == 0xFFFD || unicode.IsControl(r):
			// skip
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func splitOnPunctuation(word string) []string {
	var result []string
	var current strings.Builder
	for _, r := range word {
		if unicode.IsPunct(r) {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			result = append(result, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
