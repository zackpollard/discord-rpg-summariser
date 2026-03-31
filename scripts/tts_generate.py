#!/usr/bin/env python3
"""TTS generation wrapper supporting multiple engines.

Calls the selected TTS engine as a subprocess and reports progress on stderr.
Supported engines: zipvoice, f5tts

Usage:
    python scripts/tts_generate.py \
        --engine f5tts \
        --ref-wav path/to/reference.wav \
        --ref-text "Transcription of reference audio." \
        --text "Text to synthesize." \
        --output path/to/output.wav
"""

import argparse
import os
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--engine", default="zipvoice", choices=["zipvoice"])
    parser.add_argument("--ref-wav", required=True)
    parser.add_argument("--ref-text", required=True)
    parser.add_argument("--text", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--steps", type=int, default=32)
    parser.add_argument("--speed", type=float, default=1.0)
    parser.add_argument("--seed", type=int, default=666)
    parser.add_argument("--threads", type=int, default=4)
    args = parser.parse_args()

    os.makedirs(os.path.dirname(os.path.abspath(args.output)), exist_ok=True)

    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_dir = os.path.dirname(script_dir)
    venv_bin = os.path.join(project_dir, ".venv", "bin")
    venv_python = os.path.join(venv_bin, "python")

    print("PROGRESS:0.05", file=sys.stderr, flush=True)

    if args.engine == "zipvoice":
        run_zipvoice(venv_python, project_dir, args)

    print("PROGRESS:1.00", file=sys.stderr, flush=True)
    print("DONE", file=sys.stderr, flush=True)



def run_zipvoice(venv_python, project_dir, args):
    """Run ZipVoice via its Python module."""
    zipvoice_dir = os.path.join(project_dir, ".venv", "ZipVoice")

    env = os.environ.copy()
    env["PYTHONPATH"] = zipvoice_dir

    cmd = [
        venv_python, "-m", "zipvoice.bin.infer_zipvoice",
        "--model-name", "zipvoice",
        "--prompt-wav", args.ref_wav,
        "--prompt-text", args.ref_text,
        "--text", args.text,
        "--res-wav-path", args.output,
        "--num-step", str(args.steps),
        "--speed", str(args.speed),
        "--seed", str(args.seed),
        "--num-thread", str(args.threads),
        "--remove-long-sil", "True",
    ]

    proc = subprocess.Popen(
        cmd,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )

    for line in proc.stdout:
        line = line.decode("utf-8", errors="replace").strip()
        if line:
            print(line, file=sys.stdout, flush=True)

    proc.wait()
    if proc.returncode != 0:
        print(f"ERROR: ZipVoice exited with code {proc.returncode}", file=sys.stderr, flush=True)
        sys.exit(proc.returncode)


if __name__ == "__main__":
    main()
