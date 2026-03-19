<script lang="ts">
	import { onMount } from 'svelte';
	import {
		fetchCharacters,
		upsertCharacter,
		deleteCharacter,
		type CharacterMapping
	} from '$lib/api';

	let characters = $state<CharacterMapping[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Add form
	let newUserId = $state('');
	let newGuildId = $state('');
	let newName = $state('');
	let addError = $state<string | null>(null);
	let adding = $state(false);

	// Inline edit state
	let editingUserId = $state<string | null>(null);
	let editName = $state('');
	let editSaving = $state(false);

	// Delete confirmation
	let deleteConfirm = $state<string | null>(null);

	async function loadCharacters() {
		loading = true;
		error = null;
		try {
			characters = await fetchCharacters();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load characters';
		} finally {
			loading = false;
		}
	}

	async function handleAdd() {
		if (!newUserId.trim() || !newName.trim()) {
			addError = 'User ID and character name are required';
			return;
		}
		adding = true;
		addError = null;
		try {
			await upsertCharacter(newUserId.trim(), newGuildId.trim(), newName.trim());
			newUserId = '';
			newGuildId = '';
			newName = '';
			await loadCharacters();
		} catch (e) {
			addError = e instanceof Error ? e.message : 'Failed to add character';
		} finally {
			adding = false;
		}
	}

	function startEdit(mapping: CharacterMapping) {
		editingUserId = mapping.user_id;
		editName = mapping.character_name;
	}

	function cancelEdit() {
		editingUserId = null;
		editName = '';
	}

	async function saveEdit(mapping: CharacterMapping) {
		if (!editName.trim()) return;
		editSaving = true;
		try {
			await upsertCharacter(mapping.user_id, mapping.guild_id, editName.trim());
			editingUserId = null;
			editName = '';
			await loadCharacters();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update character';
		} finally {
			editSaving = false;
		}
	}

	async function handleDelete(userId: string) {
		try {
			await deleteCharacter(userId);
			deleteConfirm = null;
			await loadCharacters();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete character';
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		});
	}

	onMount(() => {
		loadCharacters();
	});
</script>

<svelte:head>
	<title>Characters - RPG Summariser</title>
</svelte:head>

<div class="characters-page">
	<h1>Character Mappings</h1>
	<p class="subtitle">Map Discord user IDs to character names for transcript display.</p>

	<section class="add-form card">
		<h2>Add Mapping</h2>
		<form onsubmit={(e) => { e.preventDefault(); handleAdd(); }}>
			<div class="form-row">
				<div class="field">
					<label for="userId">Discord User ID</label>
					<input
						id="userId"
						type="text"
						bind:value={newUserId}
						placeholder="e.g. 123456789012345678"
					/>
				</div>
				<div class="field">
					<label for="guildId">Guild ID <span class="optional">(optional)</span></label>
					<input
						id="guildId"
						type="text"
						bind:value={newGuildId}
						placeholder="Uses default if empty"
					/>
				</div>
				<div class="field">
					<label for="charName">Character Name</label>
					<input
						id="charName"
						type="text"
						bind:value={newName}
						placeholder="e.g. Tharivol Starweaver"
					/>
				</div>
				<button type="submit" class="btn-primary" disabled={adding}>
					{adding ? 'Adding...' : 'Add'}
				</button>
			</div>
			{#if addError}
				<p class="field-error">{addError}</p>
			{/if}
		</form>
	</section>

	{#if error}
		<div class="error-box">{error}</div>
	{/if}

	{#if loading}
		<p class="muted">Loading characters...</p>
	{:else if characters.length === 0}
		<div class="empty-state">
			<p>No character mappings yet.</p>
			<p class="muted">Add a mapping above to get started.</p>
		</div>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>User ID</th>
						<th>Character Name</th>
						<th>Updated</th>
						<th>Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each characters as mapping (mapping.user_id)}
						<tr>
							<td class="mono">{mapping.user_id}</td>
							<td>
								{#if editingUserId === mapping.user_id}
									<div class="inline-edit">
										<input
											type="text"
											bind:value={editName}
											onkeydown={(e) => {
												if (e.key === 'Enter') saveEdit(mapping);
												if (e.key === 'Escape') cancelEdit();
											}}
										/>
										<button class="btn-sm btn-primary" onclick={() => saveEdit(mapping)} disabled={editSaving}>
											Save
										</button>
										<button class="btn-sm" onclick={cancelEdit}>Cancel</button>
									</div>
								{:else}
									{mapping.character_name}
								{/if}
							</td>
							<td class="nowrap">{formatDate(mapping.updated_at)}</td>
							<td class="actions">
								{#if editingUserId !== mapping.user_id}
									<button class="btn-sm" onclick={() => startEdit(mapping)}>Edit</button>
									{#if deleteConfirm === mapping.user_id}
										<button class="btn-sm btn-danger" onclick={() => handleDelete(mapping.user_id)}>Confirm</button>
										<button class="btn-sm" onclick={() => (deleteConfirm = null)}>No</button>
									{:else}
										<button class="btn-sm btn-danger" onclick={() => (deleteConfirm = mapping.user_id)}>Delete</button>
									{/if}
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.characters-page h1 {
		color: var(--accent-gold);
		margin-bottom: 0.25rem;
		font-size: 1.5rem;
	}
	.subtitle {
		color: var(--text-muted);
		font-size: 0.9rem;
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
	}

	.form-row {
		display: flex;
		gap: 0.75rem;
		align-items: flex-end;
		flex-wrap: wrap;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		flex: 1;
		min-width: 150px;
	}
	label {
		font-size: 0.8rem;
		color: var(--text-muted);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.optional {
		font-weight: 400;
		text-transform: none;
	}
	input {
		background: var(--bg-dark);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.5rem 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
		font-family: inherit;
	}
	input:focus {
		outline: none;
		border-color: var(--accent-gold-dim);
	}
	input::placeholder {
		color: var(--text-muted);
	}

	.field-error {
		color: #fca5a5;
		font-size: 0.85rem;
		margin-top: 0.5rem;
	}

	.table-wrap {
		overflow-x: auto;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th {
		text-align: left;
		padding: 0.75rem 1rem;
		font-size: 0.8rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 600;
		border-bottom: 1px solid var(--border);
	}
	td {
		padding: 0.75rem 1rem;
		font-size: 0.9rem;
		border-top: 1px solid var(--border);
	}
	.mono {
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.85rem;
	}
	.nowrap {
		white-space: nowrap;
	}
	.actions {
		display: flex;
		gap: 0.4rem;
		white-space: nowrap;
	}

	.inline-edit {
		display: flex;
		gap: 0.4rem;
		align-items: center;
	}
	.inline-edit input {
		padding: 0.3rem 0.5rem;
		font-size: 0.9rem;
		width: 180px;
	}

	.btn-primary {
		background: var(--accent-gold-dim);
		border: 1px solid var(--accent-gold);
		color: var(--bg-dark);
		padding: 0.5rem 1rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.85rem;
		font-weight: 600;
		transition: background 0.15s;
		white-space: nowrap;
	}
	.btn-primary:hover:not(:disabled) {
		background: var(--accent-gold);
	}
	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-sm {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.25rem 0.6rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.8rem;
		transition: background 0.15s, border-color 0.15s;
	}
	.btn-sm:hover {
		background: var(--surface-hover);
		border-color: var(--accent-gold-dim);
	}

	.btn-danger {
		border-color: #7f1d1d;
		color: #fca5a5;
	}
	.btn-danger:hover {
		background: rgba(185, 28, 28, 0.2);
		border-color: #b91c1c;
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
		margin-bottom: 1rem;
	}
</style>
