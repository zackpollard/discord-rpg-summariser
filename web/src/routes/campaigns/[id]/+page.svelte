<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchStatus, fetchSessions, subscribeVoiceActivity, subscribeLiveTranscript, campaignPDFURL, type Status, type Session, type VoiceUser, type LiveTranscriptEvent } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	const campaignId = $derived(Number($page.params.id));

	let status = $state<Status | null>(null);
	let sessions = $state<Session[]>([]);
	let voiceUsers = $state<VoiceUser[]>([]);
	let liveSegments = $state<LiveTranscriptEvent[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let pollTimer: ReturnType<typeof setInterval> | undefined;
	let unsubVoice: (() => void) | undefined;
	let unsubTranscript: (() => void) | undefined;
	let transcriptEl: HTMLDivElement;

	async function loadData() {
		loading = true;
		error = null;
		try {
			const [s, sess] = await Promise.all([
				fetchStatus(),
				fetchSessions(5, 0, campaignId)
			]);
			status = s;
			sessions = sess;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	async function loadStatus() {
		try { status = await fetchStatus(); } catch { }
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit'
		});
	}

	function formatTimestamp(seconds: number): string {
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = Math.floor(seconds % 60);
		if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function charColor(name: string): string {
		let hash = 0;
		for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
		return `hsl(${((hash % 360) + 360) % 360}, 70%, 65%)`;
	}

	function formatPackets(count: number): string {
		if (count > 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
		if (count > 1000) return `${(count / 1000).toFixed(1)}k`;
		return String(count);
	}

	onMount(() => {
		loadData();
		pollTimer = setInterval(loadStatus, 5000);
		unsubVoice = subscribeVoiceActivity((users) => { voiceUsers = users; });
		unsubTranscript = subscribeLiveTranscript((seg) => {
			liveSegments = [...liveSegments, seg];
			if (liveSegments.length > 1000) liveSegments = liveSegments.slice(-1000);
			requestAnimationFrame(() => {
				if (transcriptEl) transcriptEl.scrollTop = transcriptEl.scrollHeight;
			});
		});
		return () => {
			if (pollTimer) clearInterval(pollTimer);
			if (unsubVoice) unsubVoice();
			if (unsubTranscript) unsubTranscript();
		};
	});
</script>

<svelte:head>
	<title>Campaign Dashboard - RPG Summariser</title>
</svelte:head>

<section class="status-card">
	<h2>Recording Status</h2>
	{#if status}
		<div class="status-indicator" class:active={status.recording}>
			<span class="dot"></span>
			<span>{status.recording ? 'Recording in progress' : 'Idle'}</span>
		</div>
		{#if status.active_session}
			<div class="active-session">
				<p>Session started {formatDate(status.active_session.started_at)}</p>
				<a href="/sessions/{status.active_session.id}">View session</a>
			</div>
		{/if}
	{:else if loading}
		<p class="muted">Loading...</p>
	{/if}
</section>

{#if voiceUsers.length > 0}
	<section class="card">
		<h2>Voice Channel</h2>
		<div class="voice-list">
			{#each voiceUsers as user (user.user_id)}
				<div class="voice-user" class:speaking={user.speaking} class:status-handshaking={user.status === 'handshaking' || user.status === 'reconnecting'} class:status-failed={user.status === 'decrypt_failed'}>
					<span class="voice-dot" class:active={user.speaking}></span>
					<span class="voice-name">{user.display_name || user.user_id}</span>
					{#if user.status === 'handshaking' || user.status === 'reconnecting'}
						<span class="voice-status voice-status-warn" title={user.status_message || ''}>
							{user.status === 'reconnecting' ? 'reconnecting…' : 'handshaking…'}
							{#if user.lost_packets > 0}({user.lost_packets} lost){/if}
						</span>
					{:else if user.status === 'decrypt_failed'}
						<span class="voice-status voice-status-error" title={user.status_message || ''}>decrypt failed</span>
					{:else if user.status === 'active'}
						<span class="voice-status voice-status-ok">active{#if user.lost_packets > 0} ({user.lost_packets} lost){/if}</span>
					{/if}
					<span class="voice-packets">{formatPackets(user.packet_count)} pkts</span>
				</div>
			{/each}
		</div>
	</section>
{/if}

{#if liveSegments.length > 0}
	<section class="card">
		<h2>Live Transcript</h2>
		<div class="transcript-scroll" bind:this={transcriptEl}>
			{#each liveSegments as seg}
				<div class="live-line">
					<span class="live-time">[{formatTimestamp(seg.start_time)}]</span>
					<span class="live-speaker" style="color: {charColor(seg.display_name || seg.user_id)}">{seg.display_name || seg.user_id}:</span>
					<span class="live-text">{seg.text}</span>
				</div>
			{/each}
		</div>
	</section>
{/if}

<section class="card pdf-section">
	<div class="section-header">
		<h2>Campaign Book</h2>
		<a href={campaignPDFURL(campaignId)} class="pdf-btn" download>Download PDF</a>
	</div>
	<p class="muted">Generate a D&D-style PDF campaign book with session recaps, entity compendium, quest log, and stats.</p>
</section>

<section class="card">
	<div class="section-header">
		<h2>Recent Sessions</h2>
		<a href="/campaigns/{campaignId}/sessions" class="view-all">View all</a>
	</div>
	{#if loading}
		<p class="muted">Loading sessions...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if sessions.length === 0}
		<p class="muted">No sessions recorded yet. Use <code>/session start</code> in Discord.</p>
	{:else}
		<div class="session-list">
			{#each sessions as session (session.id)}
				<a href="/sessions/{session.id}" class="session-row">
					<div class="session-info">
						<span class="session-date">{formatDate(session.started_at)}</span>
						<StatusBadge status={session.status} />
					</div>
					{#if session.title}
						<p class="session-title-preview">{session.title}</p>
					{/if}
					{#if session.summary}
						<p class="session-summary">{session.summary.slice(0, 120)}{session.summary.length > 120 ? '...' : ''}</p>
					{:else}
						<p class="session-summary muted">No summary available</p>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</section>

<style>
	.card, .status-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.25rem;
	}
	.card h2, .status-card h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}

	.status-indicator { display: flex; align-items: center; gap: 0.5rem; }
	.dot { width: 10px; height: 10px; border-radius: 50%; background: #525252; flex-shrink: 0; }
	.status-indicator.active .dot {
		background: #ef4444;
		box-shadow: 0 0 8px rgba(239, 68, 68, 0.6);
		animation: pulse 1.5s infinite;
	}
	@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
	.active-session { margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px solid var(--border); font-size: 0.9rem; }
	.active-session p { color: var(--text-secondary); margin-bottom: 0.25rem; }

	.voice-list { display: flex; flex-direction: column; gap: 0.5rem; }
	.voice-user { display: flex; align-items: center; gap: 0.75rem; padding: 0.5rem 0.75rem; background: var(--bg-surface-2); border: 1px solid var(--border); border-radius: var(--radius); }
	.voice-user.speaking { border-color: #22c55e; }
	.voice-user.status-handshaking { border-color: #eab308; }
	.voice-user.status-failed { border-color: #ef4444; }
	.voice-dot { width: 8px; height: 8px; border-radius: 50%; background: #525252; }
	.voice-dot.active { background: #22c55e; box-shadow: 0 0 6px rgba(34, 197, 94, 0.6); }
	.voice-name { font-family: monospace; font-size: 0.85rem; flex: 1; }
	.voice-packets { color: var(--text-muted); font-size: 0.75rem; font-family: monospace; }
	.voice-status { font-size: 0.7rem; padding: 0.1rem 0.4rem; border-radius: 999px; text-transform: uppercase; letter-spacing: 0.02em; font-weight: 600; }
	.voice-status-ok { background: rgba(34, 197, 94, 0.15); color: #22c55e; }
	.voice-status-warn { background: rgba(234, 179, 8, 0.15); color: #eab308; }
	.voice-status-error { background: rgba(239, 68, 68, 0.15); color: #ef4444; }

	.transcript-scroll { max-height: 400px; overflow-y: auto; font-family: monospace; font-size: 0.85rem; line-height: 1.6; padding: 0.5rem; background: var(--bg-surface-2); border-radius: var(--radius); }
	.live-line { padding: 0.15rem 0; }
	.live-time { color: var(--text-muted); margin-right: 0.5rem; }
	.live-speaker { font-weight: 600; margin-right: 0.4rem; }
	.live-text { color: var(--text-primary); }

	.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
	.section-header h2 { margin-bottom: 0; }
	.view-all { font-size: 0.85rem; }

	.session-list { display: flex; flex-direction: column; gap: 0.5rem; }
	.session-row { display: block; padding: 0.75rem; background: var(--bg-surface-2); border: 1px solid var(--border); border-radius: var(--radius); }
	.session-row:hover { background: var(--surface-hover); border-color: var(--accent-gold-dim); text-decoration: none; }
	.session-info { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.35rem; }
	.session-date { font-weight: 500; font-size: 0.9rem; }
	.session-title-preview { color: var(--accent-gold); font-weight: 600; font-size: 0.85rem; margin: 0; }
	.session-summary { color: var(--text-secondary); font-size: 0.85rem; line-height: 1.4; }

	.pdf-section p { font-size: 0.85rem; line-height: 1.4; }
	.pdf-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border: 1px solid var(--accent-gold);
		padding: 0.4rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s;
		text-decoration: none;
	}
	.pdf-btn:hover { background: var(--accent-gold); text-decoration: none; }

	.muted { color: var(--text-muted); }
	.error-box { background: rgba(185, 28, 28, 0.15); border: 1px solid #7f1d1d; color: #fca5a5; padding: 0.75rem; border-radius: var(--radius); font-size: 0.9rem; }
	code { background: var(--bg-surface-2); padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.85rem; }
</style>
