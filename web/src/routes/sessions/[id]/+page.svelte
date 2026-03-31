<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchSession, fetchTranscript, fetchSessionCombat, fetchLLMLogs, reprocessSession, deleteSession, sessionAudioURL, subscribePipelineProgress, type Session, type TranscriptSegment, type CombatEncounter, type PipelineProgressEvent, type LLMLog } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import TranscriptLine from '$lib/components/TranscriptLine.svelte';
	import AudioPlayer from '$lib/components/AudioPlayer.svelte';

	let session = $state<Session | null>(null);
	let transcript = $state<TranscriptSegment[]>([]);
	let combatEncounters = $state<CombatEncounter[]>([]);
	let expandedEncounters = $state<Set<number>>(new Set());
	let loading = $state(true);
	let error = $state<string | null>(null);
	let reprocessing = $state(false);
	let reprocessMessage = $state<string | null>(null);
	let showDeleteConfirm = $state(false);
	let deleting = $state(false);
	let audioCurrentTime = $state(0);
	let audioPlayer = $state<AudioPlayer | null>(null);
	let transcriptScrollEl = $state<HTMLDivElement | null>(null);
	let userScrolling = $state(false);
	let userScrollTimer: ReturnType<typeof setTimeout> | null = null;

	// LLM debug logs state.
	let llmLogs = $state<LLMLog[]>([]);
	let showDebugLogs = $state(false);
	let expandedLogs = $state<Set<number>>(new Set());

	function toggleLog(id: number) {
		const next = new Set(expandedLogs);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		expandedLogs = next;
	}

	async function loadDebugLogs() {
		if (!session || llmLogs.length > 0) return;
		try {
			llmLogs = await fetchLLMLogs(session.id);
		} catch { }
	}

	// Pipeline progress state.
	let progressEvent = $state<PipelineProgressEvent | null>(null);
	let liveSegments = $state<{ speaker: string; text: string; start_time: number; end_time: number }[]>([]);
	let unsubProgress: (() => void) | null = null;

	function subscribeToProgress(sessionId: number) {
		unsubProgress?.();
		liveSegments = [];
		progressEvent = null;
		unsubProgress = subscribePipelineProgress(
			sessionId,
			(evt) => {
				if (evt.type === 'progress') {
					progressEvent = evt;
				} else if (evt.type === 'transcript' && evt.speaker && evt.text) {
					liveSegments = [...liveSegments, {
						speaker: evt.speaker,
						text: evt.text,
						start_time: evt.start_time ?? 0,
						end_time: evt.end_time ?? 0
					}];
				} else if (evt.type === 'complete') {
					progressEvent = null;
					// Reload session data now that processing is done.
					reloadSession(sessionId);
				}
			},
			() => {
				// idle — no pipeline running
				progressEvent = null;
			}
		);
	}

	async function reloadSession(sessionId: number) {
		try {
			const [sess, trans, combat] = await Promise.all([
				fetchSession(sessionId),
				fetchTranscript(sessionId),
				fetchSessionCombat(sessionId)
			]);
			session = sess;
			transcript = trans;
			combatEncounters = combat;
			liveSegments = [];
			reprocessMessage = null;
			// Refresh debug logs if the panel is open.
			if (showDebugLogs) {
				llmLogs = await fetchLLMLogs(sessionId);
			} else {
				llmLogs = []; // force re-fetch on next open
			}
		} catch { }
	}

	// Find the transcript segment that contains the current playback time.
	const activeSegmentId = $derived.by(() => {
		if (transcript.length === 0) return null;
		for (const seg of transcript) {
			if (seg.start_time <= audioCurrentTime && audioCurrentTime < seg.end_time) {
				return seg.id;
			}
		}
		return null;
	});

	// Auto-scroll to the active segment when it changes (unless user is scrolling).
	$effect(() => {
		const id = activeSegmentId;
		if (id === null || userScrolling || !transcriptScrollEl) return;
		const el = transcriptScrollEl.querySelector(`[data-seg-id="${id}"]`);
		if (el) {
			el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
		}
	});

	function handleTranscriptScroll() {
		userScrolling = true;
		if (userScrollTimer) clearTimeout(userScrollTimer);
		userScrollTimer = setTimeout(() => {
			userScrolling = false;
		}, 3000);
	}

	function handleSegmentClick(startTime: number) {
		if (audioPlayer) {
			audioPlayer.seekTo(startTime);
		}
	}

	function toggleEncounter(id: number) {
		const next = new Set(expandedEncounters);
		if (next.has(id)) {
			next.delete(id);
		} else {
			next.add(id);
		}
		expandedEncounters = next;
	}

	function formatTime(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		return `${m}:${s.toString().padStart(2, '0')}`;
	}

	function actionTypeLabel(type: string): string {
		const labels: Record<string, string> = {
			attack: 'Attack',
			spell: 'Spell',
			ability: 'Ability',
			heal: 'Heal',
			damage_taken: 'Damage Taken',
			save: 'Save',
			skill: 'Skill'
		};
		return labels[type] || type;
	}

	function actionTypeClass(type: string): string {
		const classes: Record<string, string> = {
			attack: 'action-attack',
			spell: 'action-spell',
			ability: 'action-ability',
			heal: 'action-heal',
			damage_taken: 'action-damage',
			save: 'action-save',
			skill: 'action-skill'
		};
		return classes[type] || '';
	}

	async function handleDelete() {
		if (!session) return;
		deleting = true;
		try {
			await deleteSession(session.id);
			goto(`/campaigns/${session.campaign_id}/sessions`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete session';
			deleting = false;
			showDeleteConfirm = false;
		}
	}

	async function handleReprocess(retranscribe: boolean) {
		if (!session) return;
		reprocessing = true;
		reprocessMessage = null;
		try {
			await reprocessSession(session.id, retranscribe);
			reprocessing = false;
			subscribeToProgress(session.id);
		} catch (e) {
			reprocessMessage = e instanceof Error ? e.message : 'Failed to start reprocessing';
			reprocessing = false;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			weekday: 'long',
			day: 'numeric',
			month: 'long',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function formatDuration(start: string, end: string | null): string {
		if (!end) return 'In progress';
		const ms = new Date(end).getTime() - new Date(start).getTime();
		const mins = Math.floor(ms / 60000);
		const hours = Math.floor(mins / 60);
		const remainMins = mins % 60;
		if (hours > 0) return `${hours}h ${remainMins}m`;
		return `${mins}m`;
	}

	const summaryParagraphs = $derived(
		session?.summary?.split('\n').filter((p) => p.trim()) ?? []
	);

	onMount(() => {
		const id = Number($page.params.id);
		if (isNaN(id)) {
			error = 'Invalid session ID';
			loading = false;
			return;
		}

		Promise.all([fetchSession(id), fetchTranscript(id), fetchSessionCombat(id)])
			.then(async ([sess, trans, combat]) => {
				session = sess;
				transcript = trans;
				combatEncounters = combat;
				loading = false;

				// Subscribe to progress if session is still being processed.
				if (sess.status !== 'complete' && sess.status !== 'failed') {
					subscribeToProgress(sess.id);
				}

				// After data loads and DOM renders, scroll to the hash fragment
				// (e.g. #seg-123 from transcript search links).
				if (window.location.hash) {
					await tick();
					const el = document.querySelector(window.location.hash);
					if (el) {
						el.classList.add('highlighted');
						el.scrollIntoView({ behavior: 'smooth', block: 'center' });
					}
				}
			})
			.catch((e) => {
				error = e instanceof Error ? e.message : 'Failed to load session';
				loading = false;
			});
	});

	onDestroy(() => {
		unsubProgress?.();
	});
</script>

<svelte:head>
	<title>Session {$page.params.id} - RPG Summariser</title>
</svelte:head>

<div class="session-detail">
	{#if session}
		<a href="/campaigns/{session.campaign_id}/sessions" class="back-link">&larr; Back to sessions</a>
	{:else}
		<a href="/" class="back-link">&larr; Back</a>
	{/if}

	{#if loading}
		<p class="muted">Loading session...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if session}
		<div class="header">
			<h1>Session #{session.id}</h1>
			<StatusBadge status={session.status} />
		</div>

		<div class="meta">
			<div class="meta-item">
				<span class="meta-label">Started</span>
				<span>{formatDate(session.started_at)}</span>
			</div>
			<div class="meta-item">
				<span class="meta-label">Duration</span>
				<span>{formatDuration(session.started_at, session.ended_at)}</span>
			</div>
			<div class="meta-item">
				<span class="meta-label">Channel</span>
				<span class="mono">{session.channel_id}</span>
			</div>
		</div>

		{#if progressEvent}
			<div class="progress-panel">
				<div class="progress-header">
					<span class="progress-stage">{progressEvent.detail || progressEvent.stage}</span>
					<span class="progress-pct">{Math.round(progressEvent.percent)}%</span>
				</div>
				<div class="progress-bar-track">
					<div class="progress-bar-fill" style="width: {progressEvent.percent}%"></div>
				</div>
				{#if progressEvent.eta_seconds > 0}
					<span class="progress-eta">
						~{progressEvent.eta_seconds < 60
							? `${Math.round(progressEvent.eta_seconds)}s`
							: `${Math.round(progressEvent.eta_seconds / 60)}m`} remaining
					</span>
				{/if}
				{#if liveSegments.length > 0}
					<div class="live-transcript-preview">
						<h3>Live Transcript</h3>
						<div class="live-transcript-scroll">
							{#each liveSegments as seg}
								<div class="live-seg">
									<span class="live-seg-speaker">{seg.speaker}</span>
									<span class="live-seg-text">{seg.text}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			</div>
		{:else if session.status !== 'complete' && session.status !== 'failed'}
			<div class="processing-notice">
				This session is still being processed ({session.status}). Content may be incomplete.
			</div>
		{/if}

		<div class="reprocess-actions">
			<button
				class="btn btn-secondary"
				disabled={reprocessing}
				onclick={() => handleReprocess(false)}
			>
				{reprocessing ? 'Processing...' : 'Re-run Summary & Extraction'}
			</button>
			<button
				class="btn btn-secondary"
				disabled={reprocessing}
				onclick={() => handleReprocess(true)}
			>
				{reprocessing ? 'Processing...' : 'Re-run Full Pipeline (incl. Transcription)'}
			</button>
			<button
				class="btn btn-danger"
				disabled={deleting}
				onclick={() => showDeleteConfirm = true}
			>
				Delete Session
			</button>
			{#if reprocessMessage}
				<span class="reprocess-message">{reprocessMessage}</span>
			{/if}
		</div>

		{#if showDeleteConfirm}
			<div class="delete-confirm-box">
				<p>Are you sure you want to permanently delete <strong>Session #{session.id}</strong>? This will remove all transcripts, notes, combat data, and embeddings associated with this session. This cannot be undone.</p>
				<div class="delete-confirm-actions">
					<button class="btn btn-danger-solid" disabled={deleting} onclick={handleDelete}>
						{deleting ? 'Deleting...' : 'Yes, delete'}
					</button>
					<button class="btn btn-secondary" onclick={() => showDeleteConfirm = false}>Cancel</button>
				</div>
			</div>
		{/if}

		{#if summaryParagraphs.length > 0}
			<section class="card">
				<h2>Summary</h2>
				<div class="summary-body">
					{#each summaryParagraphs as paragraph}
						<p>{paragraph}</p>
					{/each}
				</div>
			</section>
		{/if}

		{#if session.key_events.length > 0}
			<section class="card">
				<h2>Key Events</h2>
				<ul class="events-list">
					{#each session.key_events as event}
						<li>{event}</li>
					{/each}
				</ul>
			</section>
		{/if}

		{#if combatEncounters.length > 0}
			<section class="card combat-section">
				<h2>Combat Encounters</h2>
				{#each combatEncounters as encounter (encounter.id)}
					<div class="encounter-card">
						<button class="encounter-header" onclick={() => toggleEncounter(encounter.id)}>
							<span class="encounter-indicator"></span>
							<span class="encounter-name">{encounter.name}</span>
							<span class="encounter-time">{formatTime(encounter.start_time)} - {formatTime(encounter.end_time)}</span>
							<span class="encounter-toggle">{expandedEncounters.has(encounter.id) ? '\u25B2' : '\u25BC'}</span>
						</button>
						{#if encounter.summary}
							<p class="encounter-summary">{encounter.summary}</p>
						{/if}
						{#if expandedEncounters.has(encounter.id)}
							<div class="actions-list">
								{#if encounter.actions.length === 0}
									<p class="muted">No actions recorded.</p>
								{:else}
									<table class="actions-table">
										<thead>
											<tr>
												<th>Round</th>
												<th>Actor</th>
												<th>Type</th>
												<th>Target</th>
												<th>Detail</th>
												<th>Damage</th>
											</tr>
										</thead>
										<tbody>
											{#each encounter.actions as action (action.id)}
												<tr>
													<td class="action-round">{action.round ?? '-'}</td>
													<td class="action-actor">{action.actor}</td>
													<td><span class="action-type-badge {actionTypeClass(action.action_type)}">{actionTypeLabel(action.action_type)}</span></td>
													<td>{action.target || '-'}</td>
													<td class="action-detail">{action.detail}</td>
													<td class="action-damage">{action.damage != null ? action.damage : '-'}</td>
												</tr>
											{/each}
										</tbody>
									</table>
								{/if}
							</div>
						{/if}
					</div>
				{/each}
			</section>
		{/if}

		<AudioPlayer
			bind:this={audioPlayer}
			src={sessionAudioURL(session.id)}
			bind:currentTime={audioCurrentTime}
		/>

		<section class="card transcript-section">
			<h2>Transcript</h2>
			{#if transcript.length === 0}
				<p class="muted">No transcript segments available.</p>
			{:else}
				<div
					class="transcript-scroll"
					bind:this={transcriptScrollEl}
					onscroll={handleTranscriptScroll}
				>
					{#each transcript as segment (segment.id)}
						<div data-seg-id={segment.id}>
							<TranscriptLine
								{segment}
								active={activeSegmentId === segment.id}
								onclick={() => handleSegmentClick(segment.start_time)}
							/>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="debug-section">
			<button
				class="debug-toggle"
				onclick={() => { showDebugLogs = !showDebugLogs; if (showDebugLogs) loadDebugLogs(); }}
			>
				<span class="debug-toggle-icon">{showDebugLogs ? '\u25B2' : '\u25BC'}</span>
				LLM Debug Logs
			</button>
			{#if showDebugLogs}
				{#if llmLogs.length === 0}
					<p class="muted" style="padding: 0.5rem 0;">No LLM logs recorded for this session.</p>
				{:else}
					<div class="debug-logs">
						{#each llmLogs as log (log.id)}
							<div class="debug-log-entry" class:debug-log-error={!!log.error}>
								<button class="debug-log-header" onclick={() => toggleLog(log.id)}>
									<span class="debug-log-op">{log.operation}</span>
									<span class="debug-log-duration">{(log.duration_ms / 1000).toFixed(1)}s</span>
									{#if log.error}
										<span class="debug-log-error-badge">error</span>
									{/if}
									<span class="debug-log-time">{new Date(log.created_at).toLocaleTimeString()}</span>
									<span class="encounter-toggle">{expandedLogs.has(log.id) ? '\u25B2' : '\u25BC'}</span>
								</button>
								{#if expandedLogs.has(log.id)}
									<div class="debug-log-body">
										{#if log.error}
											<div class="debug-log-block debug-log-block-error">
												<h4>Error</h4>
												<pre>{log.error}</pre>
											</div>
										{/if}
										<div class="debug-log-block">
											<h4>Prompt <span class="debug-log-size">({(log.prompt.length / 1024).toFixed(1)} KB)</span></h4>
											<pre>{log.prompt}</pre>
										</div>
										<div class="debug-log-block">
											<h4>Response <span class="debug-log-size">({(log.response.length / 1024).toFixed(1)} KB)</span></h4>
											<pre>{log.response}</pre>
										</div>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		</section>
	{/if}
</div>

<style>
	.session-detail {
		max-width: 900px;
	}

	.back-link {
		display: inline-block;
		margin-bottom: 1rem;
		font-size: 0.9rem;
		color: var(--text-muted);
	}
	.back-link:hover {
		color: var(--accent-gold);
	}

	.header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}
	.header h1 {
		color: var(--accent-gold);
		font-size: 1.5rem;
	}

	.meta {
		display: flex;
		gap: 2rem;
		flex-wrap: wrap;
		margin-bottom: 1.25rem;
		padding: 0.75rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.meta-item {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}
	.meta-label {
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		font-weight: 600;
	}
	.meta-item span:last-child {
		font-size: 0.9rem;
	}
	.mono {
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.85rem;
	}

	.progress-panel {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem 1.25rem;
		margin-bottom: 1.25rem;
	}
	.progress-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.5rem;
	}
	.progress-stage {
		font-size: 0.85rem;
		color: var(--text-primary);
		font-weight: 500;
	}
	.progress-pct {
		font-size: 0.85rem;
		color: var(--accent-gold);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.progress-bar-track {
		height: 6px;
		background: var(--bg-dark);
		border-radius: 3px;
		overflow: hidden;
		margin-bottom: 0.4rem;
	}
	.progress-bar-fill {
		height: 100%;
		background: var(--accent-gold);
		border-radius: 3px;
		transition: width 0.4s ease;
	}
	.progress-eta {
		font-size: 0.75rem;
		color: var(--text-muted);
	}
	.live-transcript-preview {
		margin-top: 0.75rem;
		border-top: 1px solid var(--border);
		padding-top: 0.75rem;
	}
	.live-transcript-preview h3 {
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		font-weight: 600;
		margin-bottom: 0.5rem;
	}
	.live-transcript-scroll {
		max-height: 200px;
		overflow-y: auto;
		padding: 0.4rem;
		background: var(--bg-dark);
		border-radius: var(--radius);
		border: 1px solid var(--border);
	}
	.live-seg {
		padding: 0.2rem 0;
		font-size: 0.8rem;
		line-height: 1.4;
	}
	.live-seg-speaker {
		color: var(--accent-gold-dim);
		font-weight: 600;
		margin-right: 0.4rem;
	}
	.live-seg-speaker::after {
		content: ':';
	}
	.live-seg-text {
		color: var(--text-primary);
	}

	.processing-notice {
		background: rgba(161, 98, 7, 0.15);
		border: 1px solid #92400e;
		color: #fcd34d;
		padding: 0.6rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		margin-bottom: 1.25rem;
	}

	.reprocess-actions {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
		margin-bottom: 1.25rem;
	}
	.btn {
		padding: 0.5rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid var(--border);
		transition: background 0.15s, border-color 0.15s;
	}
	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.btn-secondary {
		background: var(--bg-surface);
		color: var(--text-primary);
	}
	.btn-secondary:hover:not(:disabled) {
		background: var(--border);
		border-color: var(--text-muted);
	}
	.btn-danger {
		background: var(--bg-surface);
		color: #f87171;
		border-color: rgba(239, 68, 68, 0.3);
	}
	.btn-danger:hover:not(:disabled) {
		background: rgba(185, 28, 28, 0.2);
		border-color: rgba(239, 68, 68, 0.5);
	}
	.btn-danger-solid {
		background: rgba(185, 28, 28, 0.6);
		color: #fca5a5;
		border-color: rgba(185, 28, 28, 0.8);
	}
	.btn-danger-solid:hover:not(:disabled) {
		background: rgba(185, 28, 28, 0.8);
	}
	.delete-confirm-box {
		background: rgba(185, 28, 28, 0.1);
		border: 1px solid rgba(185, 28, 28, 0.3);
		border-radius: var(--radius);
		padding: 0.75rem 1rem;
		margin-bottom: 1.25rem;
	}
	.delete-confirm-box p {
		color: var(--text-primary);
		font-size: 0.85rem;
		margin-bottom: 0.5rem;
		line-height: 1.5;
	}
	.delete-confirm-actions {
		display: flex;
		gap: 0.5rem;
	}
	.reprocess-message {
		font-size: 0.85rem;
		color: var(--accent-gold);
	}

	.card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.25rem;
	}
	.card h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--border);
	}

	.summary-body p {
		margin-bottom: 0.75rem;
		line-height: 1.7;
		color: var(--text-primary);
	}
	.summary-body p:last-child {
		margin-bottom: 0;
	}

	.events-list {
		list-style: none;
		padding: 0;
	}
	.events-list li {
		padding: 0.4rem 0 0.4rem 1.25rem;
		position: relative;
		color: var(--text-primary);
		font-size: 0.9rem;
		line-height: 1.5;
	}
	.events-list li::before {
		content: '\25C6';
		position: absolute;
		left: 0;
		color: var(--accent-gold-dim);
		font-size: 0.6rem;
		top: 0.6rem;
	}

	/* Combat section styles */
	.combat-section {
		margin-bottom: 1.25rem;
	}

	.encounter-card {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		margin-bottom: 0.75rem;
		background: var(--bg-dark);
		overflow: hidden;
	}
	.encounter-card:last-child {
		margin-bottom: 0;
	}

	.encounter-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		padding: 0.75rem 1rem;
		background: none;
		border: none;
		color: var(--text-primary);
		cursor: pointer;
		font-size: 0.9rem;
		text-align: left;
	}
	.encounter-header:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.encounter-indicator {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #ef4444;
		flex-shrink: 0;
	}

	.encounter-name {
		font-weight: 600;
		color: var(--accent-gold);
		flex: 1;
	}

	.encounter-time {
		font-size: 0.8rem;
		color: var(--text-muted);
		font-family: 'Courier New', Courier, monospace;
	}

	.encounter-toggle {
		color: var(--text-muted);
		font-size: 0.7rem;
	}

	.encounter-summary {
		padding: 0 1rem 0.75rem;
		font-size: 0.85rem;
		color: var(--text-secondary);
		line-height: 1.5;
		margin: 0;
	}

	.actions-list {
		padding: 0 0.75rem 0.75rem;
	}

	.actions-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8rem;
	}
	.actions-table th {
		text-align: left;
		padding: 0.4rem 0.5rem;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		border-bottom: 1px solid var(--border);
		font-weight: 600;
	}
	.actions-table td {
		padding: 0.35rem 0.5rem;
		color: var(--text-primary);
		border-bottom: 1px solid rgba(255, 255, 255, 0.03);
	}
	.actions-table tr:last-child td {
		border-bottom: none;
	}

	.action-round {
		color: var(--text-muted);
		text-align: center;
		width: 3rem;
	}
	.action-actor {
		font-weight: 500;
	}
	.action-detail {
		color: var(--text-secondary);
		max-width: 200px;
	}
	.action-damage {
		text-align: center;
		font-weight: 600;
		width: 4rem;
	}

	.action-type-badge {
		display: inline-block;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}
	.action-attack { background: rgba(239, 68, 68, 0.2); color: #fca5a5; }
	.action-spell { background: rgba(139, 92, 246, 0.2); color: #c4b5fd; }
	.action-ability { background: rgba(59, 130, 246, 0.2); color: #93c5fd; }
	.action-heal { background: rgba(34, 197, 94, 0.2); color: #86efac; }
	.action-damage { background: rgba(249, 115, 22, 0.2); color: #fdba74; }
	.action-save { background: rgba(234, 179, 8, 0.2); color: #fde047; }
	.action-skill { background: rgba(20, 184, 166, 0.2); color: #5eead4; }

	.transcript-scroll {
		max-height: 500px;
		overflow-y: auto;
		padding: 0.5rem;
		background: var(--bg-dark);
		border-radius: var(--radius);
		border: 1px solid var(--border);
	}
	.transcript-scroll::-webkit-scrollbar {
		width: 6px;
	}
	.transcript-scroll::-webkit-scrollbar-track {
		background: var(--bg-dark);
	}
	.transcript-scroll::-webkit-scrollbar-thumb {
		background: var(--border);
		border-radius: 3px;
	}

	.muted {
		color: var(--text-muted);
	}
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}

	/* Debug logs */
	.debug-section {
		margin-bottom: 1.25rem;
	}
	.debug-toggle {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		background: none;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-muted);
		padding: 0.5rem 1rem;
		font-size: 0.8rem;
		cursor: pointer;
		width: 100%;
		text-align: left;
	}
	.debug-toggle:hover {
		color: var(--text-secondary);
		border-color: var(--text-muted);
	}
	.debug-toggle-icon {
		font-size: 0.65rem;
	}
	.debug-logs {
		margin-top: 0.5rem;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.debug-log-entry {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-surface);
		overflow: hidden;
	}
	.debug-log-entry.debug-log-error {
		border-color: rgba(239, 68, 68, 0.3);
	}
	.debug-log-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		padding: 0.6rem 1rem;
		background: none;
		border: none;
		color: var(--text-primary);
		cursor: pointer;
		font-size: 0.8rem;
		text-align: left;
	}
	.debug-log-header:hover {
		background: rgba(255, 255, 255, 0.03);
	}
	.debug-log-op {
		font-weight: 600;
		color: var(--text-secondary);
		min-width: 120px;
	}
	.debug-log-duration {
		color: var(--text-muted);
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.75rem;
	}
	.debug-log-error-badge {
		background: rgba(239, 68, 68, 0.2);
		color: #fca5a5;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
		font-size: 0.65rem;
		font-weight: 600;
		text-transform: uppercase;
	}
	.debug-log-time {
		color: var(--text-muted);
		font-size: 0.75rem;
		margin-left: auto;
	}
	.debug-log-body {
		padding: 0 1rem 1rem;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.debug-log-block {
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.debug-log-block-error {
		border-color: rgba(239, 68, 68, 0.3);
	}
	.debug-log-block h4 {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		font-weight: 600;
		padding: 0.5rem 0.75rem;
		margin: 0;
		border-bottom: 1px solid var(--border);
		background: rgba(255, 255, 255, 0.02);
	}
	.debug-log-block-error h4 {
		color: #fca5a5;
	}
	.debug-log-size {
		font-weight: 400;
		text-transform: none;
		letter-spacing: 0;
	}
	.debug-log-block pre {
		margin: 0;
		padding: 0.75rem;
		font-size: 0.75rem;
		line-height: 1.5;
		color: var(--text-primary);
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 400px;
		overflow-y: auto;
	}
</style>
