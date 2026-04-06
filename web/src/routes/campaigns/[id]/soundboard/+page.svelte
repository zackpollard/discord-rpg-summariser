<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchClips, deleteClip, playClipInVoice, clipAudioURL, fetchStatus, type SoundboardClip, type Status } from '$lib/api';
	import AudioPlayer from '$lib/components/AudioPlayer.svelte';

	const campaignId = $derived(Number($page.params.id));

	let clips = $state<SoundboardClip[]>([]);
	let loading = $state(true);
	let botInVoice = $state(false);

	async function loadClips() {
		loading = true;
		try {
			clips = await fetchClips(campaignId);
		} catch {}
		loading = false;
	}

	async function handleDelete(clipId: number) {
		try {
			await deleteClip(clipId);
			clips = clips.filter(c => c.id !== clipId);
		} catch {}
	}

	async function handlePlay(clipId: number) {
		try {
			await playClipInVoice(clipId);
		} catch {}
	}

	function formatDuration(start: number, end: number): string {
		const d = end - start;
		if (d < 60) return `${d.toFixed(1)}s`;
		const m = Math.floor(d / 60);
		const s = Math.floor(d % 60);
		return `${m}m ${s}s`;
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric'
		});
	}

	onMount(async () => {
		loadClips();
		try {
			const status: Status = await fetchStatus();
			botInVoice = status.recording;
		} catch {}
	});
</script>

<svelte:head>
	<title>Soundboard - RPG Summariser</title>
</svelte:head>

<div class="soundboard-page">
	{#if loading}
		<p class="muted">Loading clips...</p>
	{:else if clips.length === 0}
		<div class="empty-state">
			<p>No soundboard clips yet.</p>
			<p class="muted">Create clips from transcript segments on the session detail page.</p>
		</div>
	{:else}
		<div class="clips-grid">
			{#each clips as clip (clip.id)}
				<div class="clip-card">
					<div class="clip-header">
						<span class="clip-name">{clip.name}</span>
						<span class="clip-meta">{formatDuration(clip.start_time, clip.end_time)}</span>
					</div>
					<AudioPlayer src={clipAudioURL(clip.id)} />
					<div class="clip-actions">
						<a href={clipAudioURL(clip.id)} class="clip-action" download="{clip.name}.wav">Download</a>
						{#if botInVoice}
							<button class="clip-action clip-action-play" onclick={() => handlePlay(clip.id)}>Play in Voice</button>
						{/if}
						<button class="clip-action clip-action-delete" onclick={() => handleDelete(clip.id)}>Delete</button>
					</div>
					<div class="clip-footer">
						<span class="clip-date">{formatDate(clip.created_at)}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.soundboard-page {
		max-width: 900px;
	}

	.empty-state {
		text-align: center;
		padding: 3rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}

	.clips-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 1rem;
	}

	.clip-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.clip-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.clip-name {
		font-weight: 600;
		color: var(--accent-gold);
		font-size: 0.95rem;
	}

	.clip-meta {
		font-size: 0.8rem;
		color: var(--text-muted);
		font-family: 'Courier New', Courier, monospace;
	}

	.clip-actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.clip-action {
		padding: 0.3rem 0.6rem;
		font-size: 0.75rem;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--bg-dark);
		color: var(--text-secondary);
		cursor: pointer;
		text-decoration: none;
		font-weight: 500;
	}
	.clip-action:hover {
		border-color: var(--text-muted);
		color: var(--text-primary);
	}
	.clip-action-play {
		border-color: rgba(34, 197, 94, 0.3);
		color: #86efac;
	}
	.clip-action-play:hover {
		border-color: rgba(34, 197, 94, 0.6);
	}
	.clip-action-delete {
		border-color: rgba(239, 68, 68, 0.3);
		color: #fca5a5;
	}
	.clip-action-delete:hover {
		border-color: rgba(239, 68, 68, 0.5);
	}

	.clip-footer {
		display: flex;
		justify-content: flex-end;
	}

	.clip-date {
		font-size: 0.7rem;
		color: var(--text-muted);
	}

	.muted { color: var(--text-muted); }
</style>
