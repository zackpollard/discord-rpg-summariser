<script lang="ts">
	import { createClip, suggestClipNames } from '$lib/api';
	import WaveformTimeline from './WaveformTimeline.svelte';

	let {
		sessionId,
		campaignId,
		startTime = $bindable(0),
		endTime = $bindable(0),
		users,
		sessionDuration = 0,
		transcriptExcerpt = '',
		onclose,
		oncreated
	}: {
		sessionId: number;
		campaignId: number;
		startTime: number;
		endTime: number;
		users: { user_id: string; display_name: string }[];
		sessionDuration?: number;
		transcriptExcerpt?: string;
		onclose: () => void;
		oncreated?: () => void;
	} = $props();

	let name = $state('');
	let selectedUsers = $state<Set<string>>(new Set(users.map(u => u.user_id)));
	let creating = $state(false);
	let error = $state<string | null>(null);

	// Name suggestion state.
	let suggestions = $state<string[]>([]);
	let suggesting = $state(false);

	async function handleSuggestNames() {
		if (!transcriptExcerpt) return;
		suggesting = true;
		try {
			const result = await suggestClipNames(transcriptExcerpt);
			suggestions = result.suggestions;
		} catch { }
		suggesting = false;
	}

	function toggleUser(uid: string) {
		const next = new Set(selectedUsers);
		if (next.has(uid)) next.delete(uid);
		else next.add(uid);
		selectedUsers = next;
	}

	function formatTime(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		const ms = Math.round((seconds % 1) * 10);
		return `${m}:${String(s).padStart(2, '0')}.${ms}`;
	}

	async function handleCreate() {
		if (!name.trim() || selectedUsers.size === 0) return;
		creating = true;
		error = null;
		try {
			await createClip(campaignId, {
				session_id: sessionId,
				name: name.trim(),
				start_time: startTime,
				end_time: endTime,
				user_ids: Array.from(selectedUsers)
			});
			if (oncreated) oncreated();
			onclose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create clip';
		} finally {
			creating = false;
		}
	}
</script>

<div class="clip-overlay" onclick={onclose}>
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="clip-modal" onclick={(e) => e.stopPropagation()}>
		<h3>Create Clip</h3>

		<WaveformTimeline
			peaksUrl={`/api/sessions/${sessionId}/waveform`}
			bind:startTime={startTime}
			bind:endTime={endTime}
			duration={sessionDuration}
		/>

		<div class="clip-field">
			<label>Name</label>
			<div class="clip-name-row">
				<input type="text" bind:value={name} placeholder="e.g. Dragon roar, Epic moment" />
				{#if transcriptExcerpt}
					<button class="clip-btn clip-btn-suggest" onclick={handleSuggestNames} disabled={suggesting}>
						{suggesting ? '...' : 'Suggest'}
					</button>
				{/if}
			</div>
			{#if suggestions.length > 0}
				<div class="clip-suggestions">
					{#each suggestions as suggestion}
						<button class="clip-chip" onclick={() => { name = suggestion; }}>{suggestion}</button>
					{/each}
				</div>
			{/if}
		</div>

		<div class="clip-row">
			<div class="clip-field">
				<label>Start ({formatTime(startTime)})</label>
				<input type="number" bind:value={startTime} step="0.01" min="0" />
			</div>
			<div class="clip-field">
				<label>End ({formatTime(endTime)})</label>
				<input type="number" bind:value={endTime} step="0.01" min={startTime} />
			</div>
			<div class="clip-field">
				<label>Duration</label>
				<span class="clip-duration">{formatTime(endTime - startTime)}</span>
			</div>
		</div>

		<div class="clip-field">
			<label>Include audio from:</label>
			<div class="clip-users">
				{#each users as user}
					<label class="clip-user-check">
						<input
							type="checkbox"
							checked={selectedUsers.has(user.user_id)}
							onchange={() => toggleUser(user.user_id)}
						/>
						{user.display_name}
					</label>
				{/each}
			</div>
		</div>

		{#if error}
			<div class="clip-error">{error}</div>
		{/if}

		<div class="clip-actions">
			<button class="clip-btn clip-btn-primary" onclick={handleCreate} disabled={creating || !name.trim() || selectedUsers.size === 0}>
				{creating ? 'Creating...' : 'Create Clip'}
			</button>
			<button class="clip-btn" onclick={onclose}>Cancel</button>
		</div>
	</div>
</div>

<style>
	.clip-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}
	.clip-modal {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.5rem;
		width: 90%;
		max-width: 500px;
	}
	.clip-modal h3 {
		margin: 0 0 1rem;
		color: var(--accent-gold);
		font-size: 1.1rem;
	}
	.clip-field {
		margin-bottom: 0.75rem;
	}
	.clip-field label {
		display: block;
		font-size: 0.8rem;
		color: var(--text-muted);
		margin-bottom: 0.25rem;
		font-weight: 500;
	}
	.clip-field input[type="text"],
	.clip-field input[type="number"] {
		width: 100%;
		padding: 0.4rem 0.6rem;
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		font-size: 0.85rem;
		box-sizing: border-box;
	}
	.clip-row {
		display: flex;
		gap: 0.75rem;
	}
	.clip-row .clip-field {
		flex: 1;
	}
	.clip-duration {
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.9rem;
		color: var(--text-primary);
	}
	.clip-users {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.clip-user-check {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		font-size: 0.85rem;
		color: var(--text-primary);
		cursor: pointer;
	}
	.clip-error {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.5rem;
		border-radius: var(--radius);
		font-size: 0.8rem;
		margin-bottom: 0.75rem;
	}
	.clip-actions {
		display: flex;
		gap: 0.5rem;
		justify-content: flex-end;
		margin-top: 1rem;
	}
	.clip-btn {
		padding: 0.45rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid var(--border);
		background: var(--bg-dark);
		color: var(--text-primary);
	}
	.clip-btn:hover { border-color: var(--text-muted); }
	.clip-btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.clip-name-row {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}
	.clip-name-row input {
		flex: 1;
	}
	.clip-btn-suggest {
		white-space: nowrap;
		padding: 0.4rem 0.75rem;
		font-size: 0.8rem;
	}
	.clip-suggestions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.35rem;
		margin-top: 0.4rem;
	}
	.clip-chip {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: 1rem;
		color: var(--text-secondary);
		padding: 0.2rem 0.6rem;
		font-size: 0.75rem;
		cursor: pointer;
		transition: all 0.15s;
	}
	.clip-chip:hover {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}
	.clip-btn-primary {
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border-color: var(--accent-gold);
	}
	.clip-btn-primary:hover:not(:disabled) { background: var(--accent-gold); }
</style>
