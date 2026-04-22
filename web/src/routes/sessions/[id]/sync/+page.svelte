<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import {
		fetchSessionSync,
		saveSessionSync,
		clearSessionSync,
		remixSession,
		fetchSessionWaveform,
		fetchUserWaveform,
		sessionAudioURL,
		userAudioURL,
		type UserOffset,
		type WaveformResponse
	} from '$lib/api';

	const sessionId = $derived(Number($page.params.id));

	let users = $state<UserOffset[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);
	let remixing = $state(false);
	let statusMsg = $state<string | null>(null);

	// Shared timeline state.
	let fullDuration = $state(0);
	let viewStart = $state(0);
	let viewEnd = $state(0);

	// Per-track waveform data (keyed by userID).
	let userWaveforms = $state<Record<string, WaveformResponse>>({});
	let mixWaveform = $state<WaveformResponse | null>(null);

	// Per-track offset (local state, user-editable).
	let offsets = $state<Record<string, number>>({});

	// HTMLAudioElement per user and shared mix element, for sync playback.
	let audioEls: Record<string, HTMLAudioElement> = {};
	let mixEl = $state<HTMLAudioElement | null>(null);
	let playbackTime = $state(0); // seconds into session timeline

	let timelineEl: HTMLDivElement;
	const ROW_HEIGHT = 56;
	const LABEL_WIDTH = 180;

	async function load() {
		loading = true;
		error = null;
		try {
			const data = await fetchSessionSync(sessionId);
			users = data.users;
			offsets = Object.fromEntries(users.map((u) => [u.user_id, u.override_offset]));

			// Compute initial viewport = full session.
			let maxEnd = 0;
			for (const u of users) {
				const end = u.override_offset + u.duration_sec;
				if (end > maxEnd) maxEnd = end;
			}
			fullDuration = maxEnd;
			viewStart = 0;
			viewEnd = Math.max(10, maxEnd);

			await reloadWaveforms();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	async function reloadWaveforms() {
		// Load waveforms at the current zoom level for every user + the mix.
		const promises: Promise<unknown>[] = [];
		promises.push(
			fetchSessionWaveform(sessionId, viewStart, viewEnd).then((w) => {
				mixWaveform = w;
				if (w.full_duration_sec > fullDuration) fullDuration = w.full_duration_sec;
			}).catch(() => { /* mix may not exist yet */ })
		);
		for (const u of users) {
			promises.push(
				fetchUserWaveform(sessionId, u.user_id, 0, Math.max(1, u.duration_sec))
					.then((w) => {
						userWaveforms = { ...userWaveforms, [u.user_id]: w };
					})
					.catch(() => {})
			);
		}
		await Promise.all(promises);
	}

	function adjustOffset(userId: string, delta: number) {
		const next = (offsets[userId] ?? 0) + delta;
		offsets = { ...offsets, [userId]: Math.max(0, next) };
	}

	function setOffset(userId: string, val: number) {
		offsets = { ...offsets, [userId]: Math.max(0, val) };
	}

	function resetOffset(userId: string) {
		const u = users.find((u) => u.user_id === userId);
		if (u) offsets = { ...offsets, [userId]: u.auto_offset };
	}

	async function save() {
		saving = true;
		statusMsg = null;
		try {
			await saveSessionSync(sessionId, offsets);
			statusMsg = 'Saved overrides. Click "Regenerate mix" to hear the result.';
			// Reload to refresh auto vs override flags.
			const data = await fetchSessionSync(sessionId);
			users = data.users;
		} catch (e) {
			statusMsg = e instanceof Error ? e.message : 'Save failed';
		} finally {
			saving = false;
		}
	}

	async function clearOverrides() {
		if (!confirm('Clear all manual offsets and revert to auto-detected values?')) return;
		saving = true;
		statusMsg = null;
		try {
			await clearSessionSync(sessionId);
			statusMsg = 'Cleared overrides.';
			await load();
		} catch (e) {
			statusMsg = e instanceof Error ? e.message : 'Clear failed';
		} finally {
			saving = false;
		}
	}

	async function doRemix() {
		remixing = true;
		statusMsg = 'Regenerating mix...';
		try {
			// Save current overrides first.
			await saveSessionSync(sessionId, offsets);
			await remixSession(sessionId);
			// Reload waveform + audio element to pick up new mix.
			await fetchSessionWaveform(sessionId, viewStart, viewEnd).then((w) => {
				mixWaveform = w;
			});
			if (mixEl) mixEl.src = `${sessionAudioURL(sessionId)}?t=${Date.now()}`;
			statusMsg = 'Remix complete.';
		} catch (e) {
			statusMsg = e instanceof Error ? e.message : 'Remix failed';
		} finally {
			remixing = false;
		}
	}

	function secondsToX(seconds: number, trackWidth: number): number {
		const range = viewEnd - viewStart;
		if (range <= 0) return 0;
		return ((seconds - viewStart) / range) * trackWidth;
	}

	function formatSec(s: number): string {
		if (!Number.isFinite(s)) return '—';
		const sign = s < 0 ? '-' : '';
		const abs = Math.abs(s);
		const m = Math.floor(abs / 60);
		const sec = abs - m * 60;
		return `${sign}${m}:${sec.toFixed(2).padStart(5, '0')}`;
	}

	function zoomFit() {
		viewStart = 0;
		viewEnd = Math.max(10, fullDuration);
		reloadWaveforms();
	}

	function zoom(factor: number) {
		const center = (viewStart + viewEnd) / 2;
		const half = ((viewEnd - viewStart) / factor) / 2;
		viewStart = Math.max(0, center - half);
		viewEnd = Math.min(fullDuration || viewEnd, center + half);
		if (viewEnd - viewStart < 1) viewEnd = viewStart + 1;
		reloadWaveforms();
	}

	function panBy(seconds: number) {
		const dur = viewEnd - viewStart;
		viewStart = Math.max(0, viewStart + seconds);
		viewEnd = viewStart + dur;
		if (fullDuration && viewEnd > fullDuration) {
			viewEnd = fullDuration;
			viewStart = Math.max(0, viewEnd - dur);
		}
		reloadWaveforms();
	}

	// Synchronized playback using a virtual clock (independent of any one track).
	let playing = $state(false);
	let playStartPerf: number | null = null;
	let playStartSec = 0;

	function syncAllTracks() {
		if (!playing) return;
		for (const u of users) {
			const a = audioEls[u.user_id];
			if (!a) continue;
			const off = offsets[u.user_id] ?? 0;
			const local = playbackTime - off;
			const inRange = local >= 0 && local < u.duration_sec;
			if (inRange) {
				if (Math.abs(a.currentTime - local) > 0.15) {
					try { a.currentTime = local; } catch { /* seek may fail before metadata ready */ }
				}
				if (a.paused) a.play().catch(() => {});
			} else if (!a.paused) {
				a.pause();
			}
		}
	}

	function togglePlay() {
		if (playing) {
			for (const a of Object.values(audioEls)) a.pause();
			playing = false;
			playStartPerf = null;
			return;
		}
		playing = true;
		playStartPerf = performance.now();
		playStartSec = playbackTime;
		syncAllTracks();
	}

	function seekTo(sessionSec: number) {
		playbackTime = Math.max(0, sessionSec);
		if (playing) {
			playStartPerf = performance.now();
			playStartSec = playbackTime;
			syncAllTracks();
		} else {
			for (const u of users) {
				const a = audioEls[u.user_id];
				if (!a) continue;
				const local = playbackTime - (offsets[u.user_id] ?? 0);
				if (local >= 0 && local < u.duration_sec) {
					try { a.currentTime = local; } catch { /* ignore */ }
				}
			}
		}
	}

	let rafId: number | null = null;
	function tickPlayback() {
		if (playing && playStartPerf !== null) {
			playbackTime = playStartSec + (performance.now() - playStartPerf) / 1000;
			let anyActive = false;
			for (const u of users) {
				const off = offsets[u.user_id] ?? 0;
				if (playbackTime < off + u.duration_sec) { anyActive = true; break; }
			}
			if (!anyActive) {
				playing = false;
				playStartPerf = null;
				for (const a of Object.values(audioEls)) a.pause();
			} else {
				syncAllTracks();
			}
		}
		rafId = requestAnimationFrame(tickPlayback);
	}

	// Drag-to-adjust state: offset (per-user track) or pan (mix / empty area).
	type DragMode =
		| { kind: 'offset'; userId: string; startX: number; startOffset: number }
		| { kind: 'pan'; startX: number; startViewStart: number; startViewEnd: number };

	let drag: DragMode | null = null;
	let dragMoved = false;
	const DRAG_THRESHOLD_PX = 3;

	function trackPxPerSec(): number {
		if (!timelineEl) return 0;
		const rect = timelineEl.getBoundingClientRect();
		const trackW = rect.width - LABEL_WIDTH;
		const span = viewEnd - viewStart;
		if (trackW <= 0 || span <= 0) return 0;
		return trackW / span;
	}

	function beginOffsetDrag(e: PointerEvent, userId: string) {
		if (e.button !== 0) return;
		const el = e.currentTarget as HTMLElement;
		el.setPointerCapture(e.pointerId);
		drag = { kind: 'offset', userId, startX: e.clientX, startOffset: offsets[userId] ?? 0 };
		dragMoved = false;
		e.preventDefault();
	}

	function beginPanDrag(e: PointerEvent) {
		if (e.button !== 0) return;
		const el = e.currentTarget as HTMLElement;
		el.setPointerCapture(e.pointerId);
		drag = { kind: 'pan', startX: e.clientX, startViewStart: viewStart, startViewEnd: viewEnd };
		dragMoved = false;
		e.preventDefault();
	}

	function onDragMove(e: PointerEvent) {
		if (!drag) return;
		const dx = e.clientX - drag.startX;
		if (Math.abs(dx) > DRAG_THRESHOLD_PX) dragMoved = true;
		const pxPerSec = trackPxPerSec();
		if (pxPerSec <= 0) return;
		const dSec = dx / pxPerSec;
		if (drag.kind === 'offset') {
			offsets = { ...offsets, [drag.userId]: Math.max(0, drag.startOffset + dSec) };
		} else {
			const width = drag.startViewEnd - drag.startViewStart;
			let ns = Math.max(0, drag.startViewStart - dSec);
			let ne = ns + width;
			if (fullDuration && ne > fullDuration) {
				ne = fullDuration;
				ns = Math.max(0, ne - width);
			}
			viewStart = ns;
			viewEnd = ne;
		}
	}

	let panReloadTimer: ReturnType<typeof setTimeout> | null = null;
	function endDrag(e: PointerEvent) {
		if (!drag) return;
		const mode = drag.kind;
		const moved = dragMoved;
		drag = null;
		if (!moved) {
			// Treat as click → seek
			const trackEl = e.currentTarget as HTMLElement;
			const rect = trackEl.getBoundingClientRect();
			const x = e.clientX - rect.left;
			if (x >= 0 && rect.width > 0) {
				seekTo(viewStart + (x / rect.width) * (viewEnd - viewStart));
			}
			return;
		}
		if (mode === 'pan') {
			if (panReloadTimer) clearTimeout(panReloadTimer);
			panReloadTimer = setTimeout(() => reloadWaveforms(), 150);
		}
	}

	function onTrackWheel(e: WheelEvent) {
		if (!e.ctrlKey && !e.metaKey) return;
		e.preventDefault();
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		const x = e.clientX - rect.left;
		const frac = rect.width > 0 ? x / rect.width : 0.5;
		const cursorSec = viewStart + frac * (viewEnd - viewStart);
		const factor = e.deltaY < 0 ? 1.25 : 0.8;
		const newSpan = (viewEnd - viewStart) / factor;
		let ns = cursorSec - frac * newSpan;
		let ne = ns + newSpan;
		if (ns < 0) { ns = 0; ne = ns + newSpan; }
		if (fullDuration && ne > fullDuration) { ne = fullDuration; ns = Math.max(0, ne - newSpan); }
		if (ne - ns < 1) ne = ns + 1;
		viewStart = ns;
		viewEnd = ne;
		if (panReloadTimer) clearTimeout(panReloadTimer);
		panReloadTimer = setTimeout(() => reloadWaveforms(), 150);
	}

	onMount(() => {
		load();
		tickPlayback();
	});

	onDestroy(() => {
		if (rafId !== null) cancelAnimationFrame(rafId);
		for (const a of Object.values(audioEls)) a.pause();
	});
</script>

<svelte:head>
	<title>Sync correction - Session {sessionId}</title>
</svelte:head>

<div class="sync-page">
	<div class="page-header">
		<a href="/sessions/{sessionId}" class="back-link">← Back to session</a>
		<h1>Track sync correction</h1>
		<p class="hint">
			Nudge each user's track left/right to align their audio against the others.
			Use the transport controls to preview playback. "Regenerate mix" rebuilds
			<code>mixed.wav</code> using your overrides.
		</p>
	</div>

	{#if loading}
		<p class="muted">Loading…</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else}
		<div class="toolbar">
			<button class="btn" onclick={() => zoom(2)}>Zoom in</button>
			<button class="btn" onclick={() => zoom(0.5)}>Zoom out</button>
			<button class="btn" onclick={zoomFit}>Fit</button>
			<button class="btn" onclick={() => panBy(-(viewEnd - viewStart) / 2)}>◀</button>
			<button class="btn" onclick={() => panBy((viewEnd - viewStart) / 2)}>▶</button>
			<span class="range-label">
				{formatSec(viewStart)} – {formatSec(viewEnd)} / {formatSec(fullDuration)}
			</span>
			<div class="spacer"></div>
			<button class="btn primary" onclick={togglePlay} disabled={users.length === 0}>
				{playing ? '⏸ Pause' : '▶ Play'}
			</button>
			<button class="btn" onclick={save} disabled={saving}>Save overrides</button>
			<button class="btn" onclick={clearOverrides} disabled={saving}>Reset all</button>
			<button class="btn primary" onclick={doRemix} disabled={remixing}>
				{remixing ? 'Regenerating…' : 'Regenerate mix'}
			</button>
		</div>

		{#if statusMsg}
			<div class="status">{statusMsg}</div>
		{/if}

		<div class="timeline" bind:this={timelineEl} role="presentation">
			<!-- Mix row (reference + pan handle) -->
			<div class="row mix-row">
				<div class="label">
					<div class="name">Mixed</div>
					<div class="meta">drag to pan · ctrl+wheel to zoom</div>
				</div>
				<div
					class="track mix-track pannable"
					onpointerdown={beginPanDrag}
					onpointermove={onDragMove}
					onpointerup={endDrag}
					onpointercancel={endDrag}
					onwheel={onTrackWheel}
					role="presentation"
				>
					{#if mixWaveform}
						<svg viewBox="0 0 {mixWaveform.peaks.length} 100" preserveAspectRatio="none">
							{#each mixWaveform.peaks as peak, i}
								<rect x={i} y={50 - peak * 50} width="1" height={peak * 100} fill="currentColor" />
							{/each}
						</svg>
					{/if}
					<div class="playhead" style="left: {((playbackTime - viewStart) / (viewEnd - viewStart)) * 100}%"></div>
				</div>
			</div>

			<!-- Per-user rows -->
			{#each users as u (u.user_id)}
				{@const off = offsets[u.user_id] ?? 0}
				{@const wf = userWaveforms[u.user_id]}
				<div class="row">
					<div class="label">
						<div class="name">{u.character_name || u.display_name || u.user_id}</div>
						<div class="meta">
							<span class="offset-val">{off.toFixed(3)}s</span>
							{#if u.has_override}
								<span class="override-badge" title="manually overridden">override</span>
							{/if}
						</div>
						<div class="offset-controls">
							<button onclick={() => adjustOffset(u.user_id, -1)} title="-1s">−1</button>
							<button onclick={() => adjustOffset(u.user_id, -0.1)} title="-0.1s">−0.1</button>
							<button onclick={() => adjustOffset(u.user_id, -0.01)} title="-0.01s">−0.01</button>
							<button onclick={() => adjustOffset(u.user_id, 0.01)} title="+0.01s">+0.01</button>
							<button onclick={() => adjustOffset(u.user_id, 0.1)} title="+0.1s">+0.1</button>
							<button onclick={() => adjustOffset(u.user_id, 1)} title="+1s">+1</button>
							<button class="reset" onclick={() => resetOffset(u.user_id)} title="reset to auto">↺</button>
						</div>
						<input
							type="number"
							step="0.001"
							value={off}
							oninput={(e) => setOffset(u.user_id, parseFloat((e.target as HTMLInputElement).value) || 0)}
							class="offset-input"
						/>
					</div>
					<div
						class="track draggable"
						onpointerdown={(e) => beginOffsetDrag(e, u.user_id)}
						onpointermove={onDragMove}
						onpointerup={endDrag}
						onpointercancel={endDrag}
						onwheel={onTrackWheel}
						role="presentation"
					>
						{#if wf && (viewEnd - viewStart) > 0 && u.duration_sec > 0}
							{@const trackStart = secondsToX(off, 1000)}
							{@const trackEnd = secondsToX(off + u.duration_sec, 1000)}
							<svg
								preserveAspectRatio="none"
								viewBox="0 0 {wf.peaks.length} 100"
								style="position: absolute; left: {(trackStart / 1000) * 100}%; width: {((trackEnd - trackStart) / 1000) * 100}%; height: 100%;"
							>
								{#each wf.peaks as peak, i}
									<rect x={i} y={50 - peak * 50} width="1" height={peak * 100} fill="currentColor" />
								{/each}
							</svg>
						{/if}
						<div class="playhead" style="left: {((playbackTime - viewStart) / (viewEnd - viewStart)) * 100}%"></div>
					</div>
					<audio
						bind:this={audioEls[u.user_id]}
						src={userAudioURL(sessionId, u.user_id)}
						preload="auto"
					></audio>
				</div>
			{/each}
		</div>

		<audio bind:this={mixEl} src={sessionAudioURL(sessionId)} preload="metadata" style="display:none"></audio>
	{/if}
</div>

<style>
	.sync-page {
		max-width: 1600px;
		margin: 0 auto;
	}
	.page-header h1 {
		margin: 0.5rem 0;
		color: var(--accent-gold);
	}
	.hint {
		color: var(--text-secondary);
		font-size: 0.9rem;
	}
	.back-link {
		font-size: 0.85rem;
		color: var(--text-muted);
	}

	.toolbar {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		margin: 1rem 0;
		flex-wrap: wrap;
	}
	.spacer { flex: 1; }
	.btn {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.4rem 0.75rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.85rem;
	}
	.btn:hover { border-color: var(--accent-gold-dim); }
	.btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.btn.primary {
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border-color: var(--accent-gold);
	}
	.range-label {
		font-family: monospace;
		font-size: 0.8rem;
		color: var(--text-muted);
	}
	.status {
		background: var(--bg-surface);
		border: 1px solid var(--accent-gold-dim);
		color: var(--accent-gold);
		padding: 0.6rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		margin-bottom: 1rem;
	}
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
	}

	.timeline {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
		user-select: none;
	}
	.row {
		display: flex;
		border-bottom: 1px solid var(--border);
		min-height: 56px;
	}
	.row:last-child { border-bottom: none; }
	.mix-row { background: var(--bg-surface-2); }

	.label {
		flex: 0 0 180px;
		padding: 0.5rem;
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		font-size: 0.75rem;
	}
	.label .name { font-weight: 600; color: var(--text-primary); font-size: 0.85rem; }
	.label .meta { color: var(--text-muted); display: flex; gap: 0.35rem; align-items: center; }
	.offset-val { font-family: monospace; color: var(--accent-gold); }
	.override-badge {
		font-size: 0.6rem;
		background: rgba(212, 175, 125, 0.2);
		color: var(--accent-gold);
		padding: 0.05rem 0.3rem;
		border-radius: 999px;
	}
	.offset-controls {
		display: flex;
		gap: 0.15rem;
		flex-wrap: wrap;
		margin-top: 0.15rem;
	}
	.offset-controls button {
		background: var(--bg-dark);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.1rem 0.25rem;
		font-size: 0.65rem;
		border-radius: 3px;
		cursor: pointer;
		min-width: 28px;
	}
	.offset-controls button:hover { border-color: var(--accent-gold-dim); }
	.offset-controls .reset { margin-left: auto; color: var(--text-muted); }
	.offset-input {
		width: 100%;
		padding: 0.1rem 0.3rem;
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: 3px;
		color: var(--text-primary);
		font-size: 0.7rem;
		font-family: monospace;
	}

	.track {
		flex: 1;
		position: relative;
		background: var(--bg-dark);
		color: var(--accent-gold-dim);
		overflow: hidden;
		touch-action: none;
	}
	.track.draggable { cursor: grab; }
	.track.draggable:active { cursor: grabbing; }
	.track.pannable { cursor: ew-resize; }
	.mix-track { color: var(--text-secondary); }
	.track svg { width: 100%; height: 100%; display: block; }
	.playhead {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 1px;
		background: #f87171;
		pointer-events: none;
	}

	.muted { color: var(--text-muted); }
</style>
