<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchRecap, regenerateRecap, campaignPDFURL, type CampaignRecap } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));

	let recap = $state<CampaignRecap | null>(null);
	let loading = $state(true);
	let generating = $state(false);
	let error = $state<string | null>(null);
	let lastN = $state<number | undefined>(undefined);

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	async function loadRecap() {
		loading = true;
		error = null;
		try {
			recap = await fetchRecap(campaignId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load recap';
		} finally {
			loading = false;
		}
	}

	async function handleRegenerate() {
		generating = true;
		error = null;
		try {
			recap = await regenerateRecap(campaignId, lastN);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to generate recap';
		} finally {
			generating = false;
		}
	}

	onMount(() => { loadRecap(); });
</script>

<svelte:head>
	<title>Recap - RPG Summariser</title>
</svelte:head>

<div class="recap-page">
	{#if loading}
		<p class="muted">Loading recap...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if !recap || !recap.recap}
		<div class="empty-state">
			<p>No recap has been generated yet.</p>
			<p class="muted">Generate a narrative recap of your campaign so far.</p>
			<div class="last-n-row">
				<label class="last-n-label">
					Last N sessions:
					<input
						class="last-n-input"
						type="number"
						min="1"
						placeholder="All"
						oninput={(e) => { const v = parseInt((e.target as HTMLInputElement).value); lastN = Number.isNaN(v) ? undefined : v; }}
					/>
				</label>
			</div>
			<button class="generate-btn" onclick={handleRegenerate} disabled={generating}>
				{#if generating}
					<span class="spinner"></span>
					Generating...
				{:else}
					Generate Recap
				{/if}
			</button>
		</div>
	{:else}
		<div class="recap-header">
			<div class="recap-meta">
				{#if recap.recap_generated_at}
					<span class="recap-date">Last generated: {formatDate(recap.recap_generated_at)}</span>
				{/if}
			</div>
			<div class="recap-actions">
				<a href={campaignPDFURL(campaignId)} class="pdf-btn" download>Download PDF</a>
				<label class="last-n-label">
					Last N sessions:
					<input
						class="last-n-input"
						type="number"
						min="1"
						placeholder="All"
						oninput={(e) => { const v = parseInt((e.target as HTMLInputElement).value); lastN = Number.isNaN(v) ? undefined : v; }}
					/>
				</label>
				<button class="regenerate-btn" onclick={handleRegenerate} disabled={generating}>
					{#if generating}
						<span class="spinner"></span>
						Regenerating...
					{:else}
						Regenerate
					{/if}
				</button>
			</div>
		</div>

		<div class="recap-body">
			{#each recap.recap.split('\n\n') as paragraph}
				{#if paragraph.trim()}
					<p>{paragraph.trim()}</p>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<style>
	.recap-page {
		max-width: 800px;
	}

	.recap-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	.recap-date {
		color: var(--text-muted);
		font-size: 0.8rem;
	}

	.recap-actions {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.last-n-row {
		margin-bottom: 0.75rem;
	}

	.last-n-label {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		color: var(--text-muted);
		font-size: 0.85rem;
	}

	.last-n-input {
		width: 5rem;
		padding: 0.3rem 0.5rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		font-size: 0.85rem;
	}
	.last-n-input::placeholder {
		color: var(--text-muted);
	}

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

	.regenerate-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.4rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		cursor: pointer;
		transition: all 0.15s;
	}
	.regenerate-btn:hover:not(:disabled) {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}
	.regenerate-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.recap-body {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 2rem;
	}
	.recap-body p {
		color: var(--text-primary);
		font-size: 1.05rem;
		line-height: 1.8;
		margin-bottom: 1.25rem;
	}
	.recap-body p:last-child {
		margin-bottom: 0;
	}

	.empty-state {
		text-align: center;
		padding: 3rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.generate-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		margin-top: 1rem;
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border: 1px solid var(--accent-gold);
		padding: 0.5rem 1.5rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s;
	}
	.generate-btn:hover:not(:disabled) { background: var(--accent-gold); }
	.generate-btn:disabled { opacity: 0.5; cursor: not-allowed; }

	.spinner {
		width: 14px;
		height: 14px;
		border: 2px solid var(--border);
		border-top-color: var(--accent-gold);
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }

	.muted { color: var(--text-muted); }
	.error-box { background: rgba(185, 28, 28, 0.15); border: 1px solid #7f1d1d; color: #fca5a5; padding: 0.75rem; border-radius: var(--radius); font-size: 0.9rem; }
</style>
