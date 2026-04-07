<script lang="ts">
	import { onMount } from 'svelte';

	let {
		peaksUrl,
		startTime = $bindable(0),
		endTime = $bindable(0),
		duration = 0
	}: {
		peaksUrl: string;
		startTime: number;
		endTime: number;
		duration: number;
	} = $props();

	let canvas = $state<HTMLCanvasElement | null>(null);
	let container = $state<HTMLDivElement | null>(null);
	let peaks = $state<number[]>([]);
	let loading = $state(true);
	let dragging = $state<'start' | 'end' | 'region' | null>(null);
	let dragStartX = 0;
	let dragStartVal = 0;
	let dragEndVal = 0;

	// Viewport: the visible time window (zoom/pan).
	let viewStart = $state(0);
	let viewEnd = $state(0);

	const HANDLE_WIDTH = 8;
	const MIN_VIEW_DURATION = 2; // minimum visible window in seconds
	const ZOOM_FACTOR = 0.15;

	function round2(v: number): number {
		return Math.round(v * 100) / 100;
	}

	// Convert time to canvas X using the current viewport.
	function timeToX(t: number, width: number): number {
		const viewDur = viewEnd - viewStart;
		if (viewDur <= 0) return 0;
		return ((t - viewStart) / viewDur) * width;
	}

	// Convert canvas X to time using the current viewport.
	function xToTime(x: number, width: number): number {
		const viewDur = viewEnd - viewStart;
		if (viewDur <= 0) return 0;
		const t = viewStart + (x / width) * viewDur;
		return round2(Math.max(0, Math.min(duration, t)));
	}

	async function loadWaveform() {
		loading = true;
		try {
			const response = await fetch(peaksUrl);
			peaks = await response.json();
		} catch (e) {
			console.error('Failed to load waveform:', e);
		}
		loading = false;
	}

	function drawWaveform() {
		if (!canvas || peaks.length === 0) return;
		const ctx = canvas.getContext('2d');
		if (!ctx) return;

		const w = canvas.width;
		const h = canvas.height;
		const mid = h / 2;
		const viewDur = viewEnd - viewStart;

		ctx.clearRect(0, 0, w, h);

		// Draw waveform bars for the visible viewport.
		const peakDur = duration / peaks.length;
		const firstPeak = Math.max(0, Math.floor(viewStart / peakDur));
		const lastPeak = Math.min(peaks.length - 1, Math.ceil(viewEnd / peakDur));

		for (let i = firstPeak; i <= lastPeak; i++) {
			const peakTime = i * peakDur;
			const x = ((peakTime - viewStart) / viewDur) * w;
			const barW = (peakDur / viewDur) * w;
			const barH = peaks[i] * mid * 0.9;

			const inSelection = peakTime >= startTime && peakTime <= endTime;
			ctx.fillStyle = inSelection ? 'rgba(212, 175, 125, 0.8)' : 'rgba(255, 255, 255, 0.2)';
			ctx.fillRect(x, mid - barH, Math.max(barW - 0.5, 0.5), barH * 2);
		}

		// Draw selection region overlay.
		const selStart = timeToX(startTime, w);
		const selEnd = timeToX(endTime, w);

		// Dim outside selection.
		ctx.fillStyle = 'rgba(0, 0, 0, 0.4)';
		if (selStart > 0) ctx.fillRect(0, 0, selStart, h);
		if (selEnd < w) ctx.fillRect(selEnd, 0, w - selEnd, h);

		// Draw handles.
		ctx.fillStyle = 'rgba(212, 175, 125, 1)';
		if (selStart >= -HANDLE_WIDTH && selStart <= w + HANDLE_WIDTH) {
			ctx.fillRect(selStart - HANDLE_WIDTH / 2, 0, HANDLE_WIDTH, h);
		}
		if (selEnd >= -HANDLE_WIDTH && selEnd <= w + HANDLE_WIDTH) {
			ctx.fillRect(selEnd - HANDLE_WIDTH / 2, 0, HANDLE_WIDTH, h);
		}

		// Draw time labels on handles.
		ctx.font = '11px Courier New';
		ctx.fillStyle = 'rgba(212, 175, 125, 1)';
		ctx.textAlign = 'center';
		if (selStart > 20 && selStart < w - 20) {
			ctx.fillText(formatTime(startTime), selStart, h - 4);
		}
		if (selEnd > 20 && selEnd < w - 20) {
			ctx.fillText(formatTime(endTime), selEnd, h - 4);
		}

		// Draw viewport time markers at edges.
		ctx.fillStyle = 'rgba(255, 255, 255, 0.3)';
		ctx.font = '10px Courier New';
		ctx.textAlign = 'left';
		ctx.fillText(formatTime(viewStart), 4, 12);
		ctx.textAlign = 'right';
		ctx.fillText(formatTime(viewEnd), w - 4, 12);
	}

	function formatTime(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function handleMouseDown(e: MouseEvent) {
		if (!canvas) return;
		const rect = canvas.getBoundingClientRect();
		const x = e.clientX - rect.left;
		const w = canvas.width;

		const startX = timeToX(startTime, w);
		const endX = timeToX(endTime, w);

		if (Math.abs(x - startX) < HANDLE_WIDTH * 2) {
			dragging = 'start';
		} else if (Math.abs(x - endX) < HANDLE_WIDTH * 2) {
			dragging = 'end';
		} else if (x > startX && x < endX) {
			dragging = 'region';
			dragStartX = x;
			dragStartVal = startTime;
			dragEndVal = endTime;
		} else {
			const clickTime = xToTime(x, w);
			if (Math.abs(clickTime - startTime) < Math.abs(clickTime - endTime)) {
				startTime = clickTime;
			} else {
				endTime = clickTime;
			}
		}
	}

	function handleMouseMove(e: MouseEvent) {
		if (!dragging || !canvas) return;
		const rect = canvas.getBoundingClientRect();
		const x = e.clientX - rect.left;
		const w = canvas.width;

		if (dragging === 'start') {
			startTime = Math.min(xToTime(x, w), endTime - 0.1);
		} else if (dragging === 'end') {
			endTime = Math.max(xToTime(x, w), startTime + 0.1);
		} else if (dragging === 'region') {
			const viewDur = viewEnd - viewStart;
			const dx = x - dragStartX;
			const dt = (dx / w) * viewDur;
			let newStart = dragStartVal + dt;
			let newEnd = dragEndVal + dt;
			const regionDur = dragEndVal - dragStartVal;

			if (newStart < 0) { newStart = 0; newEnd = regionDur; }
			if (newEnd > duration) { newEnd = duration; newStart = duration - regionDur; }

			startTime = round2(newStart);
			endTime = round2(newEnd);
		}
	}

	function handleMouseUp() {
		dragging = null;
	}

	function handleWheel(e: WheelEvent) {
		e.preventDefault();
		if (!canvas || duration <= 0) return;

		const rect = canvas.getBoundingClientRect();
		const mouseX = e.clientX - rect.left;
		const w = canvas.width;

		// Time position under the cursor.
		const cursorTime = xToTime(mouseX, w);
		const viewDur = viewEnd - viewStart;

		// Zoom in (scroll up) or out (scroll down).
		const zoomDir = e.deltaY > 0 ? 1 : -1;
		const scale = 1 + ZOOM_FACTOR * zoomDir;
		let newDur = viewDur * scale;

		// Clamp.
		if (newDur < MIN_VIEW_DURATION) newDur = MIN_VIEW_DURATION;
		if (newDur > duration) newDur = duration;

		// Keep the cursor position fixed by adjusting viewStart/viewEnd
		// proportionally around the cursor time.
		const cursorFrac = (cursorTime - viewStart) / viewDur;
		let newStart = cursorTime - cursorFrac * newDur;
		let newEnd = newStart + newDur;

		if (newStart < 0) { newStart = 0; newEnd = newDur; }
		if (newEnd > duration) { newEnd = duration; newStart = duration - newDur; }

		viewStart = round2(Math.max(0, newStart));
		viewEnd = round2(Math.min(duration, newEnd));
	}

	$effect(() => {
		if (peaks.length > 0) {
			void startTime;
			void endTime;
			void viewStart;
			void viewEnd;
			drawWaveform();
		}
	});

	$effect(() => {
		if (canvas && container) {
			const w = container.clientWidth;
			canvas.width = w;
			canvas.height = 80;
			if (peaks.length > 0) drawWaveform();
		}
	});

	// Initialize viewport to show the selection filling ~80% of the view.
	let viewInitialized = false;
	$effect(() => {
		if (duration > 0 && !viewInitialized) {
			viewInitialized = true;
			const selDur = endTime - startTime;
			if (selDur > 0 && selDur < duration) {
				const padding = selDur * 0.125; // 10% padding each side → selection fills 80%
				viewStart = round2(Math.max(0, startTime - padding));
				viewEnd = round2(Math.min(duration, endTime + padding));
			} else {
				viewStart = 0;
				viewEnd = duration;
			}
		}
	});

	onMount(() => {
		loadWaveform();
		window.addEventListener('mousemove', handleMouseMove);
		window.addEventListener('mouseup', handleMouseUp);

		let resizeObserver: ResizeObserver | null = null;
		if (container) {
			resizeObserver = new ResizeObserver(() => {
				if (canvas && container) {
					const w = container.clientWidth;
					canvas.width = w;
					canvas.height = 80;
					if (peaks.length > 0) drawWaveform();
				}
			});
			resizeObserver.observe(container);
		}

		return () => {
			window.removeEventListener('mousemove', handleMouseMove);
			window.removeEventListener('mouseup', handleMouseUp);
			resizeObserver?.disconnect();
		};
	});
</script>

<div class="waveform-container" bind:this={container}>
	{#if loading}
		<div class="waveform-loading">Loading waveform...</div>
	{:else}
		<canvas
			bind:this={canvas}
			class="waveform-canvas"
			onmousedown={handleMouseDown}
			onwheel={handleWheel}
		></canvas>
	{/if}
</div>

<style>
	.waveform-container {
		width: 100%;
		height: 80px;
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
		margin-bottom: 0.75rem;
	}
	.waveform-loading {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: var(--text-muted);
		font-size: 0.8rem;
	}
	.waveform-canvas {
		width: 100%;
		height: 100%;
		cursor: col-resize;
	}
</style>
