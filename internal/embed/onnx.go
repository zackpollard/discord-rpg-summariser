package embed

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	ortInitOnce sync.Once
	ortInitErr  error
)

func initOnnxRuntime() error {
	ortInitOnce.Do(func() {
		libPath := findOnnxRuntime()
		ort.SetSharedLibraryPath(libPath)
		ortInitErr = ort.InitializeEnvironment()
		if ortInitErr == nil {
			log.Printf("embed: ONNX Runtime initialized (lib=%s)", libPath)
		}
	})
	return ortInitErr
}

// OnnxEmbedder implements Embedder using an in-process ONNX model.
type OnnxEmbedder struct {
	tokenizer *BertTokenizer
	session   *ort.DynamicAdvancedSession
	mu        sync.Mutex
}

const embeddingMaxLen = 512

// NewOnnxEmbedder creates an in-process ONNX embedding model.
// Model files are downloaded automatically on first use.
func NewOnnxEmbedder(modelDir string, threads int) (*OnnxEmbedder, error) {
	if err := ensureModelFiles(modelDir); err != nil {
		return nil, fmt.Errorf("ensure model files: %w", err)
	}

	if err := initOnnxRuntime(); err != nil {
		return nil, fmt.Errorf("init ONNX Runtime: %w", err)
	}

	vocabPath := filepath.Join(modelDir, "vocab.txt")
	modelPath := filepath.Join(modelDir, localModel)

	tokenizer, err := NewBertTokenizer(vocabPath, embeddingMaxLen)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	// Verify model inputs/outputs.
	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("read model info: %w", err)
	}
	logModelInfo(inputs, outputs)

	// Determine input and output names from the model.
	inputNames := make([]string, len(inputs))
	for i, in := range inputs {
		inputNames[i] = in.Name
	}
	outputNames := pickOutputNames(outputs)

	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create session options: %w", err)
	}
	defer opts.Destroy()
	if threads > 0 {
		opts.SetIntraOpNumThreads(threads)
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, opts)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	log.Printf("embed: ONNX embedder ready (inputs=%v outputs=%v)", inputNames, outputNames)
	return &OnnxEmbedder{
		tokenizer: tokenizer,
		session:   session,
	}, nil
}

// Embed returns the embedding vector for a single query text.
func (e *OnnxEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.embedTexts([]string{"search_query: " + text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedBatch returns embedding vectors for multiple document texts.
func (e *OnnxEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	prefixed := make([]string, len(texts))
	for i, t := range texts {
		prefixed[i] = "search_document: " + t
	}
	return e.embedTexts(prefixed)
}

// Close releases ONNX resources.
func (e *OnnxEmbedder) Close() {
	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}
}

const hiddenSize = 768

func (e *OnnxEmbedder) embedTexts(texts []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	batchSize := int64(len(texts))

	// Tokenize all texts and find the max sequence length.
	encodings := make([]TokenizerOutput, len(texts))
	var maxSeq int64
	for i, text := range texts {
		encodings[i] = e.tokenizer.Encode(text)
		if n := int64(len(encodings[i].InputIDs)); n > maxSeq {
			maxSeq = n
		}
	}

	// Build padded batch tensors.
	totalInputs := batchSize * maxSeq
	idsData := make([]int64, totalInputs)
	typeData := make([]int64, totalInputs) // token_type_ids: all zeros
	maskData := make([]int64, totalInputs)

	for i, enc := range encodings {
		offset := int64(i) * maxSeq
		copy(idsData[offset:], enc.InputIDs)
		copy(maskData[offset:], enc.AttentionMask)
	}

	inputShape := ort.NewShape(batchSize, maxSeq)

	idsTensor, err := ort.NewTensor(inputShape, idsData)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer idsTensor.Destroy()

	typeTensor, err := ort.NewTensor(inputShape, typeData)
	if err != nil {
		return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
	}
	defer typeTensor.Destroy()

	maskTensor, err := ort.NewTensor(inputShape, maskData)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	// Output: last_hidden_state [batch, seq, 768].
	outTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(batchSize, maxSeq, hiddenSize))
	if err != nil {
		return nil, fmt.Errorf("create output tensor: %w", err)
	}
	defer outTensor.Destroy()

	// Input order must match model: input_ids, token_type_ids, attention_mask.
	if err := e.session.Run(
		[]ort.Value{idsTensor, typeTensor, maskTensor},
		[]ort.Value{outTensor},
	); err != nil {
		return nil, fmt.Errorf("ONNX inference: %w", err)
	}

	// Mean-pool over non-padding tokens, then L2-normalise.
	outData := outTensor.GetData()
	results := make([][]float32, batchSize)
	for i := int64(0); i < batchSize; i++ {
		results[i] = l2Normalize(meanPool(
			outData[i*maxSeq*hiddenSize:],
			encodings[i].AttentionMask,
			int(maxSeq),
			hiddenSize,
		))
	}

	return results, nil
}

// meanPool computes the mean of token embeddings weighted by attention mask.
func meanPool(data []float32, mask []int64, seqLen, dim int) []float32 {
	vec := make([]float32, dim)
	var count float32
	for t := 0; t < seqLen && t < len(mask); t++ {
		if mask[t] == 0 {
			continue
		}
		count++
		off := t * dim
		for h := 0; h < dim; h++ {
			vec[h] += data[off+h]
		}
	}
	if count > 0 {
		for h := range vec {
			vec[h] /= count
		}
	}
	return vec
}

// l2Normalize normalises a vector to unit length.
func l2Normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	n := float32(math.Sqrt(sum))
	if n > 0 {
		for i := range vec {
			vec[i] /= n
		}
	}
	return vec
}

// pickOutputNames selects the best output from the model. Prefers the
// pre-pooled sentence_embedding when available, otherwise falls back to
// token_embeddings (which would require manual mean pooling).
func pickOutputNames(outputs []ort.InputOutputInfo) []string {
	for _, o := range outputs {
		if o.Name == "sentence_embedding" {
			return []string{"sentence_embedding"}
		}
	}
	// Fallback: use the first output.
	if len(outputs) > 0 {
		return []string{outputs[0].Name}
	}
	return []string{"last_hidden_state"}
}

func logModelInfo(inputs, outputs []ort.InputOutputInfo) {
	var inNames, outNames []string
	for _, in := range inputs {
		inNames = append(inNames, fmt.Sprintf("%s%v", in.Name, in.Dimensions))
	}
	for _, out := range outputs {
		outNames = append(outNames, fmt.Sprintf("%s%v", out.Name, out.Dimensions))
	}
	log.Printf("embed: model inputs=%s outputs=%s", strings.Join(inNames, ", "), strings.Join(outNames, ", "))
}

func findOnnxRuntime() string {
	if p := os.Getenv("ONNXRUNTIME_LIB"); p != "" {
		return p
	}
	// Check standard system locations and the sherpa-onnx Go module cache.
	for _, p := range append(sherpaOnnxLibPaths(), []string{
		"/usr/lib/libonnxruntime.so",
		"/usr/local/lib/libonnxruntime.so",
		"/usr/lib/x86_64-linux-gnu/libonnxruntime.so",
	}...) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("LD_LIBRARY_PATH")) {
		p := filepath.Join(dir, "libonnxruntime.so")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "libonnxruntime.so"
}

// sherpaOnnxLibPaths returns candidate paths for the ONNX Runtime shared
// library bundled with the sherpa-onnx-go-linux Go module.
func sherpaOnnxLibPaths() []string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	pattern := filepath.Join(gopath, "pkg/mod/github.com/k2-fsa/sherpa-onnx-go-linux@*/lib/x86_64-unknown-linux-gnu/libonnxruntime.so")
	matches, _ := filepath.Glob(pattern)
	return matches
}
