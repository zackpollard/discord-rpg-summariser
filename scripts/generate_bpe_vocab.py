#!/usr/bin/env python3
"""Generate bpe.vocab for Parakeet TDT hot-word support in sherpa-onnx.

This extracts the SentencePiece tokenizer from the NeMo .nemo checkpoint and
writes a bpe.vocab file that sherpa-onnx needs for modified_beam_search with
hot words.

Usage:
    pip install sentencepiece huggingface_hub
    python scripts/generate_bpe_vocab.py [--model-dir models]

The script downloads only the tokenizer config (~2 KB) plus the .nemo archive
from HuggingFace, extracts the tokenizer.model, generates bpe.vocab, and
cleans up the large archive.
"""

import argparse
import os
import sys
import tarfile
import tempfile

MODEL_REPO = "nvidia/parakeet-tdt-0.6b-v3"
NEMO_FILENAME = "parakeet-tdt-0.6b-v3.nemo"
OUTPUT_DIR = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"


def main():
    parser = argparse.ArgumentParser(description="Generate bpe.vocab for Parakeet TDT hot words")
    parser.add_argument("--model-dir", default="models", help="Base model directory (default: models)")
    args = parser.parse_args()

    output_path = os.path.join(args.model_dir, OUTPUT_DIR, "bpe.vocab")

    if os.path.exists(output_path):
        print(f"bpe.vocab already exists at {output_path}")
        return

    try:
        import sentencepiece as spm
    except ImportError:
        print("Error: sentencepiece is required. Install with: pip install sentencepiece", file=sys.stderr)
        sys.exit(1)

    try:
        from huggingface_hub import hf_hub_download
    except ImportError:
        print("Error: huggingface_hub is required. Install with: pip install huggingface_hub", file=sys.stderr)
        sys.exit(1)

    print(f"Downloading {NEMO_FILENAME} from {MODEL_REPO}...")
    nemo_path = hf_hub_download(repo_id=MODEL_REPO, filename=NEMO_FILENAME)

    # Extract tokenizer.model from the .nemo archive (which is a tar file).
    print("Extracting tokenizer.model from .nemo archive...")
    tokenizer_model = None
    with tempfile.TemporaryDirectory() as tmpdir:
        with tarfile.open(nemo_path, "r") as tar:
            for member in tar.getmembers():
                if member.name.endswith("tokenizer.model"):
                    tar.extract(member, tmpdir)
                    tokenizer_model = os.path.join(tmpdir, member.name)
                    break

        if tokenizer_model is None:
            print("Error: tokenizer.model not found in .nemo archive", file=sys.stderr)
            sys.exit(1)

        # Generate bpe.vocab using sentencepiece.
        print(f"Generating {output_path}...")
        sp = spm.SentencePieceProcessor()
        sp.Load(tokenizer_model)

        os.makedirs(os.path.dirname(output_path), exist_ok=True)
        with open(output_path, "w") as f:
            for i in range(sp.GetPieceSize()):
                piece = sp.IdToPiece(i)
                score = sp.GetScore(i)
                f.write(f"{piece}\t{score}\n")

    print(f"Generated bpe.vocab with {sp.GetPieceSize()} tokens at {output_path}")


if __name__ == "__main__":
    main()
