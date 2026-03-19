<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchSession, fetchTranscript, type Session, type TranscriptSegment } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import TranscriptLine from '$lib/components/TranscriptLine.svelte';

	let session = $state<Session | null>(null);
	let transcript = $state<TranscriptSegment[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

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

		Promise.all([fetchSession(id), fetchTranscript(id)])
			.then(([sess, trans]) => {
				session = sess;
				transcript = trans;
			})
			.catch((e) => {
				error = e instanceof Error ? e.message : 'Failed to load session';
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

<svelte:head>
	<title>Session {$page.params.id} - RPG Summariser</title>
</svelte:head>

<div class="session-detail">
	<a href="/sessions" class="back-link">&larr; Back to sessions</a>

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

		{#if session.status !== 'complete' && session.status !== 'failed'}
			<div class="processing-notice">
				This session is still being processed ({session.status}). Content may be incomplete.
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

		<section class="card transcript-section">
			<h2>Transcript</h2>
			{#if transcript.length === 0}
				<p class="muted">No transcript segments available.</p>
			{:else}
				<div class="transcript-scroll">
					{#each transcript as segment (segment.id)}
						<TranscriptLine {segment} />
					{/each}
				</div>
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

	.processing-notice {
		background: rgba(161, 98, 7, 0.15);
		border: 1px solid #92400e;
		color: #fcd34d;
		padding: 0.6rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		margin-bottom: 1.25rem;
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
</style>
