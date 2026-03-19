<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchCampaigns, createCampaign, setActiveCampaign, type Campaign } from '$lib/api';

	let campaigns = $state<Campaign[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let newName = $state('');
	let newDescription = $state('');
	let creating = $state(false);
	let createError = $state<string | null>(null);

	async function loadCampaigns() {
		loading = true;
		error = null;
		try {
			campaigns = await fetchCampaigns();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load campaigns';
		} finally {
			loading = false;
		}
	}

	async function handleCreate() {
		if (!newName.trim()) return;
		creating = true;
		createError = null;
		try {
			await createCampaign(newName.trim(), newDescription.trim());
			newName = '';
			newDescription = '';
			await loadCampaigns();
		} catch (e) {
			createError = e instanceof Error ? e.message : 'Failed to create campaign';
		} finally {
			creating = false;
		}
	}

	async function handleSetActive(id: number) {
		try {
			await setActiveCampaign(id);
			await loadCampaigns();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to set active campaign';
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric'
		});
	}

	onMount(() => { loadCampaigns(); });
</script>

<svelte:head>
	<title>Campaigns - RPG Summariser</title>
</svelte:head>

<div class="campaigns-page">
	<h1>Your Campaigns</h1>

	<section class="create-card">
		<h2>Create Campaign</h2>
		<form onsubmit={(e) => { e.preventDefault(); handleCreate(); }}>
			<div class="form-row">
				<input type="text" placeholder="Campaign name" bind:value={newName} disabled={creating} />
			</div>
			<div class="form-row">
				<textarea placeholder="Description (optional)" bind:value={newDescription} disabled={creating} rows="2"></textarea>
			</div>
			<button type="submit" class="btn-primary" disabled={creating || !newName.trim()}>
				{creating ? 'Creating...' : 'Create Campaign'}
			</button>
			{#if createError}
				<div class="error-box" style="margin-top: 0.75rem;">{createError}</div>
			{/if}
		</form>
	</section>

	{#if loading}
		<p class="muted">Loading campaigns...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if campaigns.length === 0}
		<div class="empty-state">
			<p>No campaigns yet.</p>
			<p class="muted">Create your first campaign above to get started.</p>
		</div>
	{:else}
		<section class="campaign-list">
			{#each campaigns as campaign (campaign.id)}
				<a href="/campaigns/{campaign.id}" class="campaign-card" class:active={campaign.is_active}>
					<div class="card-top">
						<span class="campaign-name">{campaign.name}</span>
						{#if campaign.is_active}
							<span class="badge active-badge">Active</span>
						{/if}
					</div>
					{#if campaign.description}
						<p class="campaign-desc">{campaign.description}</p>
					{/if}
					<span class="campaign-date">Created {formatDate(campaign.created_at)}</span>
					{#if !campaign.is_active}
						<button class="btn-set-active" onclick={(e) => { e.preventDefault(); e.stopPropagation(); handleSetActive(campaign.id); }}>
							Set Active
						</button>
					{/if}
				</a>
			{/each}
		</section>
	{/if}
</div>

<style>
	.campaigns-page h1 {
		color: var(--accent-gold);
		margin-bottom: 1.25rem;
		font-size: 1.5rem;
	}

	.create-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.5rem;
	}
	.create-card h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}
	.form-row { margin-bottom: 0.75rem; }
	input, textarea {
		width: 100%;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		padding: 0.5rem 0.75rem;
		font-size: 0.9rem;
		font-family: inherit;
		resize: vertical;
	}
	input:focus, textarea:focus {
		outline: none;
		border-color: var(--accent-gold-dim);
	}

	.btn-primary {
		background: var(--accent-gold);
		color: var(--bg-dark);
		border: none;
		padding: 0.5rem 1.25rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.85rem;
		font-weight: 600;
	}
	.btn-primary:hover:not(:disabled) { opacity: 0.85; }
	.btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }

	.campaign-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 0.75rem;
	}
	.campaign-card {
		display: block;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		transition: border-color 0.15s, box-shadow 0.15s;
		text-decoration: none;
		position: relative;
	}
	.campaign-card:hover {
		border-color: var(--accent-gold-dim);
		box-shadow: 0 2px 12px rgba(212, 175, 125, 0.08);
		text-decoration: none;
	}
	.campaign-card.active {
		border-color: var(--accent-gold-dim);
	}
	.card-top {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		margin-bottom: 0.35rem;
	}
	.campaign-name {
		font-size: 1.1rem;
		font-weight: 600;
		color: var(--text-primary);
	}
	.badge {
		font-size: 0.65rem;
		padding: 0.15rem 0.5rem;
		border-radius: 999px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.active-badge {
		background: rgba(212, 175, 125, 0.15);
		color: var(--accent-gold);
		border: 1px solid var(--accent-gold-dim);
	}
	.campaign-desc {
		color: var(--text-secondary);
		font-size: 0.85rem;
		line-height: 1.4;
		margin-bottom: 0.35rem;
	}
	.campaign-date {
		font-size: 0.75rem;
		color: var(--text-muted);
	}
	.btn-set-active {
		position: absolute;
		top: 1rem;
		right: 1rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.25rem 0.6rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.75rem;
	}
	.btn-set-active:hover {
		background: var(--surface-hover);
		border-color: var(--accent-gold-dim);
	}

	.empty-state {
		text-align: center;
		padding: 3rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.muted { color: var(--text-muted); }
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}
</style>
