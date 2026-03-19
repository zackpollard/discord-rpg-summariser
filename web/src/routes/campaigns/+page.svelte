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

	onMount(() => {
		loadCampaigns();
	});
</script>

<svelte:head>
	<title>Campaigns - RPG Summariser</title>
</svelte:head>

<div class="campaigns-page">
	<h1>Campaigns</h1>

	<section class="create-card">
		<h2>Create Campaign</h2>
		<form onsubmit={(e) => { e.preventDefault(); handleCreate(); }}>
			<div class="form-row">
				<input
					type="text"
					placeholder="Campaign name"
					bind:value={newName}
					disabled={creating}
				/>
			</div>
			<div class="form-row">
				<textarea
					placeholder="Description (optional)"
					bind:value={newDescription}
					disabled={creating}
					rows="3"
				></textarea>
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
			<p class="muted">Create your first campaign above.</p>
		</div>
	{:else}
		<section class="campaign-list">
			{#each campaigns as campaign (campaign.id)}
				<div class="campaign-card" class:active={campaign.is_active}>
					<div class="campaign-header">
						<a href="/sessions?campaign={campaign.id}" class="campaign-name">
							{campaign.name}
						</a>
						{#if campaign.is_active}
							<span class="badge active-badge">Active</span>
						{/if}
					</div>
					{#if campaign.description}
						<p class="campaign-desc">{campaign.description}</p>
					{/if}
					<div class="campaign-actions">
						{#if !campaign.is_active}
							<button class="btn-secondary" onclick={() => handleSetActive(campaign.id)}>
								Set Active
							</button>
						{/if}
						<a href="/sessions?campaign={campaign.id}" class="btn-link">View Sessions</a>
					</div>
				</div>
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

	.form-row {
		margin-bottom: 0.75rem;
	}
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
		transition: opacity 0.15s;
	}
	.btn-primary:hover:not(:disabled) {
		opacity: 0.85;
	}
	.btn-primary:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.btn-secondary {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.35rem 0.85rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.8rem;
		transition: background 0.15s, border-color 0.15s;
	}
	.btn-secondary:hover {
		background: var(--surface-hover);
		border-color: var(--accent-gold-dim);
	}

	.btn-link {
		font-size: 0.8rem;
		color: var(--accent-gold);
	}

	.campaign-list {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.campaign-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem 1.25rem;
		transition: border-color 0.15s;
	}
	.campaign-card.active {
		border-color: var(--accent-gold-dim);
	}
	.campaign-card:hover {
		border-color: var(--accent-gold-dim);
	}

	.campaign-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.35rem;
	}
	.campaign-name {
		font-size: 1.05rem;
		font-weight: 600;
		color: var(--text-primary);
	}
	.campaign-name:hover {
		color: var(--accent-gold);
	}

	.badge {
		font-size: 0.7rem;
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
		margin-bottom: 0.5rem;
		line-height: 1.4;
	}

	.campaign-actions {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-top: 0.5rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
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
