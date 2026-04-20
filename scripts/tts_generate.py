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
import re
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



def patch_solver_progress(zipvoice_dir, num_steps):
    """Inject per-step logging into the ZipVoice ODE solver."""
    solver_path = os.path.join(zipvoice_dir, "zipvoice", "models", "modules", "solver.py")
    if not os.path.exists(solver_path):
        return

    with open(solver_path, "r") as f:
        content = f.read()

    # Only patch once — check if already patched.
    if "STEP_PROGRESS" in content:
        return

    # Add a print after each step in the solver loop.
    patched = content.replace(
        "x = x + v * (timesteps[step + 1] - timesteps[step])",
        'x = x + v * (timesteps[step + 1] - timesteps[step])\n'
        '            import sys; print(f"STEP_PROGRESS:{step + 1}/{num_step}", file=sys.stderr, flush=True)',
    )

    if patched != content:
        with open(solver_path, "w") as f:
            f.write(patched)


def run_zipvoice(venv_python, project_dir, args):
    """Run ZipVoice via its Python module."""
    zipvoice_dir = os.path.join(project_dir, ".venv", "ZipVoice")

    # Patch the ODE solver to emit per-step progress.
    patch_solver_progress(zipvoice_dir, args.steps)

    env = os.environ.copy()
    env["PYTHONPATH"] = zipvoice_dir
    env["PYTHONUNBUFFERED"] = "1"

    cmd = [
        venv_python, "-u", "-m", "zipvoice.bin.infer_zipvoice",
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

            # Parse step progress from patched ODE solver.
            # ZipVoice processes the entire text in one generate_sentence call,
            # so step/total directly maps to overall progress.
            step_match = re.match(r'STEP_PROGRESS:(\d+)/(\d+)', line)
            if step_match:
                step_cur = int(step_match.group(1))
                step_total = int(step_match.group(2))
                progress = step_cur / step_total
                scaled = 0.05 + progress * 0.90
                print(f"PROGRESS:{min(scaled, 0.95):.2f}", file=sys.stderr, flush=True)

    proc.wait()
    if proc.returncode != 0:
        print(f"ERROR: ZipVoice exited with code {proc.returncode}", file=sys.stderr, flush=True)
        sys.exit(proc.returncode)


if __name__ == "__main__":
    main()
