<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	let {
		src,
		currentTime = $bindable(0),
		onseek
	}: {
		src: string;
		currentTime?: number;
		onseek?: (time: number) => void;
	} = $props();

	let audioEl = $state<HTMLAudioElement | null>(null);
	let playing = $state(false);
	let duration = $state(0);
	let seekValue = $state(0);
	let seeking = $state(false);
	let playbackRate = $state(1);
	let loaded = $state(false);
	let error = $state(false);

	const speeds = [0.5, 1, 1.5, 2];

	// Reset state when src changes.
	$effect(() => {
		void src;
		playing = false;
		duration = 0;
		seekValue = 0;
		loaded = false;
		error = false;
	});

	function formatTime(seconds: number): string {
		if (!isFinite(seconds) || seconds < 0) return '0:00';
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = Math.floor(seconds % 60);
		if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function togglePlay() {
		if (!audioEl) return;
		if (playing) {
			audioEl.pause();
		} else {
			audioEl.play();
		}
	}

	function handleTimeUpdate() {
		if (!audioEl || seeking) return;
		currentTime = audioEl.currentTime;
		seekValue = audioEl.currentTime;
	}

	function handleLoadedMetadata() {
		if (!audioEl) return;
		duration = audioEl.duration;
		loaded = true;
	}

	function handleSeekStart() {
		seeking = true;
	}

	function handleSeekInput(e: Event) {
		const target = e.target as HTMLInputElement;
		seekValue = parseFloat(target.value);
	}

	function handleSeekEnd() {
		if (!audioEl) return;
		audioEl.currentTime = seekValue;
		currentTime = seekValue;
		seeking = false;
		if (onseek) onseek(seekValue);
	}

	function setSpeed(rate: number) {
		playbackRate = rate;
		if (audioEl) audioEl.playbackRate = rate;
	}

	function handleWindowSeekEnd() {
		if (seeking) handleSeekEnd();
	}

	onMount(() => {
		window.addEventListener('mouseup', handleWindowSeekEnd);
		window.addEventListener('touchend', handleWindowSeekEnd);
	});

	onDestroy(() => {
		window.removeEventListener('mouseup', handleWindowSeekEnd);
		window.removeEventListener('touchend', handleWindowSeekEnd);
	});

	export function seekTo(time: number) {
		if (!audioEl) return;
		audioEl.currentTime = time;
		currentTime = time;
		seekValue = time;
	}
</script>

<div class="audio-player">
	<audio
		bind:this={audioEl}
		src={src}
		preload="metadata"
		ontimeupdate={handleTimeUpdate}
		onloadedmetadata={handleLoadedMetadata}
		onplay={() => (playing = true)}
		onpause={() => (playing = false)}
		onended={() => (playing = false)}
		onerror={() => (error = true)}
	></audio>

	{#if error}
		<div class="audio-error">Audio unavailable for this session.</div>
	{:else}
		<div class="controls">
			<button class="play-btn" onclick={togglePlay} disabled={!loaded} aria-label={playing ? 'Pause' : 'Play'}>
				{#if playing}
					<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
						<rect x="6" y="4" width="4" height="16" rx="1" />
						<rect x="14" y="4" width="4" height="16" rx="1" />
					</svg>
				{:else}
					<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
						<polygon points="6,4 20,12 6,20" />
					</svg>
				{/if}
			</button>

			<span class="time">{formatTime(currentTime)}</span>

			<input
				type="range"
				class="seek-slider"
				min="0"
				max={duration || 0}
				step="0.1"
				value={seekValue}
				disabled={!loaded}
				onmousedown={handleSeekStart}
				ontouchstart={handleSeekStart}
				oninput={handleSeekInput}
				onmouseup={handleSeekEnd}
				ontouchend={handleSeekEnd}
			/>

			<span class="time">{formatTime(duration)}</span>

			<div class="speed-controls">
				{#each speeds as rate}
					<button
						class="speed-btn"
						class:active={playbackRate === rate}
						onclick={() => setSpeed(rate)}
					>
						{rate}x
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>

<style>
	.audio-player {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.75rem 1rem;
		margin-bottom: 1.25rem;
	}

	.audio-error {
		color: var(--text-muted);
		font-size: 0.85rem;
		text-align: center;
		padding: 0.25rem 0;
	}

	.controls {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.play-btn {
		width: 36px;
		height: 36px;
		border-radius: 50%;
		border: 1px solid var(--border);
		background: var(--bg-dark);
		color: var(--accent-gold);
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		flex-shrink: 0;
		transition: background 0.15s, border-color 0.15s;
	}
	.play-btn:hover:not(:disabled) {
		background: var(--border);
		border-color: var(--accent-gold-dim);
	}
	.play-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.time {
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.8rem;
		color: var(--text-muted);
		min-width: 3.5rem;
		text-align: center;
		flex-shrink: 0;
		user-select: none;
	}

	.seek-slider {
		flex: 1;
		height: 4px;
		-webkit-appearance: none;
		appearance: none;
		background: var(--border);
		border-radius: 2px;
		outline: none;
		cursor: pointer;
	}
	.seek-slider::-webkit-slider-thumb {
		-webkit-appearance: none;
		appearance: none;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: var(--accent-gold);
		border: none;
		cursor: pointer;
	}
	.seek-slider::-moz-range-thumb {
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: var(--accent-gold);
		border: none;
		cursor: pointer;
	}
	.seek-slider:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.speed-controls {
		display: flex;
		gap: 0.25rem;
		flex-shrink: 0;
	}

	.speed-btn {
		padding: 0.2rem 0.4rem;
		font-size: 0.7rem;
		font-weight: 600;
		border: 1px solid var(--border);
		border-radius: 4px;
		background: var(--bg-dark);
		color: var(--text-muted);
		cursor: pointer;
		transition: background 0.15s, color 0.15s, border-color 0.15s;
	}
	.speed-btn:hover {
		color: var(--text-primary);
		border-color: var(--text-muted);
	}
	.speed-btn.active {
		background: var(--accent-gold);
		color: var(--bg-dark);
		border-color: var(--accent-gold);
	}
</style>
