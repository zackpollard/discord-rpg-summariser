<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchCampaigns, updateCampaign, fetchMembers, type Campaign, type GuildMember } from '$lib/api';

	let campaign = $state<Campaign | null>(null);
	let members = $state<GuildMember[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let saveMessage = $state<string | null>(null);

	// Form fields
	let name = $state('');
	let description = $state('');
	let gameSystem = $state('');
	let dmUserId = $state<string>('');

	const commonSystems = [
		'Dungeons & Dragons',
		'Dungeons & Dragons 5th Edition',
		'Pathfinder 2e',
		'Call of Cthulhu',
		'Shadowrun',
		'Savage Worlds',
		'FATE',
		'Blades in the Dark',
		'Cyberpunk RED',
		'Mothership',
		'Vampire: The Masquerade',
	];

	function loadFormFields(c: Campaign) {
		name = c.name;
		description = c.description;
		gameSystem = c.game_system;
		dmUserId = c.dm_user_id ?? '';
	}

	async function handleSave() {
		if (!campaign || !name.trim()) return;
		saving = true;
		saveMessage = null;
		error = null;
		try {
			const updated = await updateCampaign(campaign.id, {
				name: name.trim(),
				description: description.trim(),
				game_system: gameSystem.trim() || 'Dungeons & Dragons',
				dm_user_id: dmUserId || null,
			});
			campaign = updated;
			loadFormFields(updated);
			saveMessage = 'Settings saved.';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}

	onMount(async () => {
		const campaignId = Number($page.params.id);
		try {
			const [camps, mems] = await Promise.all([
				fetchCampaigns(),
				fetchMembers().catch(() => [] as GuildMember[]),
			]);
			campaign = camps.find(c => c.id === campaignId) ?? null;
			members = mems;
			if (campaign) loadFormFields(campaign);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load campaign';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Settings - {campaign?.name ?? 'Campaign'} - RPG Summariser</title>
</svelte:head>

<div class="settings-page">
	{#if loading}
		<p class="muted">Loading settings...</p>
	{:else if error && !campaign}
		<div class="error-box">{error}</div>
	{:else if campaign}
		<h2>Campaign Settings</h2>

		<form class="settings-form" onsubmit={(e) => { e.preventDefault(); handleSave(); }}>
			<div class="field">
				<label for="name">Campaign Name</label>
				<input id="name" type="text" bind:value={name} required />
			</div>

			<div class="field">
				<label for="description">Description</label>
				<textarea id="description" bind:value={description} rows="3" placeholder="A brief description of the campaign..."></textarea>
			</div>

			<div class="field">
				<label for="game-system">Game System</label>
				<input
					id="game-system"
					type="text"
					bind:value={gameSystem}
					list="game-systems"
					placeholder="e.g. Dungeons & Dragons"
				/>
				<datalist id="game-systems">
					{#each commonSystems as sys}
						<option value={sys} />
					{/each}
				</datalist>
				<p class="field-hint">Used to contextualise the transcription engine so it better recognises system-specific terminology.</p>
			</div>

			<div class="field">
				<label for="dm">Dungeon Master</label>
				<select id="dm" bind:value={dmUserId}>
					<option value="">-- None --</option>
					{#each members as member (member.user_id)}
						<option value={member.user_id}>{member.display_name} ({member.username})</option>
					{/each}
				</select>
				<p class="field-hint">The DM's speech is labelled separately in transcripts and their Telegram messages are included.</p>
			</div>

			<div class="field info-row">
				<div class="info-item">
					<span class="info-label">Status</span>
					<span class="info-value">{campaign.is_active ? 'Active' : 'Inactive'}</span>
				</div>
				<div class="info-item">
					<span class="info-label">Created</span>
					<span class="info-value">{new Date(campaign.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'long', year: 'numeric' })}</span>
				</div>
			</div>

			{#if error}
				<div class="error-box">{error}</div>
			{/if}
			{#if saveMessage}
				<div class="save-message">{saveMessage}</div>
			{/if}

			<div class="actions">
				<button type="submit" class="btn btn-primary" disabled={saving || !name.trim()}>
					{saving ? 'Saving...' : 'Save Settings'}
				</button>
			</div>
		</form>
	{/if}
</div>

<style>
	.settings-page {
		max-width: 600px;
	}
	h2 {
		color: var(--accent-gold);
		font-size: 1.25rem;
		margin-bottom: 1.25rem;
	}

	.settings-form {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.field label {
		font-size: 0.8rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}
	.field input,
	.field textarea,
	.field select {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		padding: 0.6rem 0.75rem;
		font-size: 0.9rem;
		font-family: inherit;
	}
	.field input:focus,
	.field textarea:focus,
	.field select:focus {
		outline: none;
		border-color: var(--accent-gold-dim);
	}
	.field textarea {
		resize: vertical;
	}
	.field select {
		cursor: pointer;
	}
	.field-hint {
		font-size: 0.78rem;
		color: var(--text-muted);
		line-height: 1.4;
	}

	.info-row {
		flex-direction: row;
		gap: 2rem;
		padding: 0.75rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.info-item {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}
	.info-label {
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		font-weight: 600;
	}
	.info-value {
		font-size: 0.9rem;
		color: var(--text-primary);
	}

	.actions {
		display: flex;
		gap: 0.75rem;
	}
	.btn {
		padding: 0.5rem 1.25rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		border: 1px solid var(--border);
		transition: background 0.15s, border-color 0.15s;
	}
	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.btn-primary {
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border-color: var(--accent-gold);
	}
	.btn-primary:hover:not(:disabled) {
		background: var(--accent-gold);
	}

	.save-message {
		font-size: 0.85rem;
		color: var(--accent-gold);
	}
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}
	.muted {
		color: var(--text-muted);
	}
</style>
