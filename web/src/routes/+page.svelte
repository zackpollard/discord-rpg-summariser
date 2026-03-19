<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchStatus, fetchSessions, subscribeVoiceActivity, type Status, type Session, type VoiceUser } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let status = $state<Status | null>(null);
	let sessions = $state<Session[]>([]);
	let voiceUsers = $state<VoiceUser[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let pollTimer: ReturnType<typeof setInterval> | undefined;
	let unsubVoice: (() => void) | undefined;

	async function loadStatus() {
		try {
			status = await fetchStatus();
		} catch (e) {
			console.warn('Failed to fetch status:', e);
		}
	}

	async function loadData() {
		loading = true;
		error = null;
		try {
			const [s, sess] = await Promise.all([fetchStatus(), fetchSessions(5, 0)]);
			status = s;
			sessions = sess;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function formatPackets(count: number): string {
		if (count > 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
		if (count > 1000) return `${(count / 1000).toFixed(1)}k`;
		return String(count);
	}

	onMount(() => {
		loadData();
		pollTimer = setInterval(loadStatus, 5000);
		unsubVoice = subscribeVoiceActivity((users) => {
			voiceUsers = users;
		});
		return () => {
			if (pollTimer) clearInterval(pollTimer);
			if (unsubVoice) unsubVoice();
		};
	});
</script>

<svelte:head>
	<title>Dashboard - RPG Summariser</title>
</svelte:head>

<div class="dashboard">
	<h1>Dashboard</h1>

	<section class="status-card">
		<h2>Recording Status</h2>
		{#if status}
			<div class="status-indicator" class:active={status.recording}>
				<span class="dot"></span>
				<span>{status.recording ? 'Recording in progress' : 'Idle'}</span>
			</div>
			{#if status.active_session}
				<div class="active-session">
					<p>
						Active session started {formatDate(status.active_session.started_at)}
					</p>
					<a href="/sessions/{status.active_session.id}">View session</a>
				</div>
			{/if}
		{:else if loading}
			<p class="muted">Loading...</p>
		{:else}
			<p class="muted">Unable to fetch status</p>
		{/if}
	</section>

	{#if voiceUsers.length > 0}
		<section class="voice-card">
			<h2>Voice Channel</h2>
			<div class="voice-list">
				{#each voiceUsers as user (user.user_id)}
					<div class="voice-user" class:speaking={user.speaking}>
						<span class="voice-dot" class:active={user.speaking}></span>
						<span class="voice-name">{user.user_id}</span>
						<span class="voice-packets">{formatPackets(user.packet_count)} pkts</span>
					</div>
				{/each}
			</div>
		</section>
	{/if}

	<section class="recent-section">
		<div class="section-header">
			<h2>Recent Sessions</h2>
			<a href="/sessions" class="view-all">View all</a>
		</div>

		{#if loading}
			<p class="muted">Loading sessions...</p>
		{:else if error}
			<div class="error-box">{error}</div>
		{:else if sessions.length === 0}
			<p class="muted">No sessions recorded yet.</p>
		{:else}
			<div class="session-list">
				{#each sessions as session (session.id)}
					<a href="/sessions/{session.id}" class="session-row">
						<div class="session-info">
							<span class="session-date">{formatDate(session.started_at)}</span>
							<StatusBadge status={session.status} />
						</div>
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
</div>

<style>
	.dashboard h1 {
		color: var(--accent-gold);
		margin-bottom: 1.25rem;
		font-size: 1.5rem;
	}

	.status-card, .voice-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.5rem;
	}
	.status-card h2, .voice-card h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}

	.status-indicator {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 1rem;
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: #525252;
		flex-shrink: 0;
	}
	.status-indicator.active .dot {
		background: #ef4444;
		box-shadow: 0 0 8px rgba(239, 68, 68, 0.6);
		animation: pulse 1.5s infinite;
	}
	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.5; }
	}

	.active-session {
		margin-top: 0.75rem;
		padding-top: 0.75rem;
		border-top: 1px solid var(--border);
		font-size: 0.9rem;
	}
	.active-session p {
		color: var(--text-secondary);
		margin-bottom: 0.25rem;
	}

	.voice-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.voice-user {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 0.75rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		transition: border-color 0.15s, box-shadow 0.15s;
	}
	.voice-user.speaking {
		border-color: #22c55e;
		box-shadow: inset 0 0 12px rgba(34, 197, 94, 0.08);
	}
	.voice-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #525252;
		flex-shrink: 0;
		transition: background 0.15s, box-shadow 0.15s;
	}
	.voice-dot.active {
		background: #22c55e;
		box-shadow: 0 0 6px rgba(34, 197, 94, 0.6);
	}
	.voice-name {
		color: var(--text-primary);
		font-family: monospace;
		font-size: 0.85rem;
		flex: 1;
	}
	.voice-packets {
		color: var(--text-muted);
		font-size: 0.75rem;
		font-family: monospace;
	}

	.recent-section {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
	}
	.section-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
	}
	.section-header h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		font-weight: 600;
	}
	.view-all {
		font-size: 0.85rem;
	}

	.session-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.session-row {
		display: block;
		padding: 0.75rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		transition: background 0.15s, border-color 0.15s;
	}
	.session-row:hover {
		background: var(--surface-hover);
		border-color: var(--accent-gold-dim);
		text-decoration: none;
	}
	.session-info {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.35rem;
	}
	.session-date {
		color: var(--text-primary);
		font-weight: 500;
		font-size: 0.9rem;
	}
	.session-summary {
		color: var(--text-secondary);
		font-size: 0.85rem;
		line-height: 1.4;
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
</style>
