<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchRecap, regenerateRecap, fetchPreviouslyOn, campaignPDFURL, fetchRecapVoices, recapTTSURL, uploadVoiceProfile, deleteVoiceProfile, type CampaignRecap, type RecapVoice, type PreviouslyOnResult } from '$lib/api';
	import AudioPlayer from '$lib/components/AudioPlayer.svelte';

	const campaignId = $derived(Number($page.params.id));

	let recap = $state<CampaignRecap | null>(null);
	let loading = $state(true);
	let generating = $state(false);
	let error = $state<string | null>(null);
	let lastN = $state<number | undefined>(undefined);
	let recapStyle = $state('default');

	// Previously On state.
	let previouslyOn = $state<PreviouslyOnResult | null>(null);
	let prevOnLoading = $state(false);
	let prevOnError = $state<string | null>(null);

	async function handleGeneratePreviouslyOn() {
		prevOnLoading = true;
		prevOnError = null;
		try {
			previouslyOn = await fetchPreviouslyOn(campaignId);
		} catch (e) {
			prevOnError = e instanceof Error ? e.message : 'Failed to generate';
		} finally {
			prevOnLoading = false;
		}
	}

	// TTS voice picker state.
	let voices = $state<RecapVoice[]>([]);
	let selectedVoiceIdx = $state<number>(-1);
	let ttsAudioSrc = $state<string>('');
	let ttsGenerating = $state(false);
	let ttsProgress = $state(0);

	// Voice profile upload state.
	let showUpload = $state(false);
	let uploadName = $state('');
	let uploadTranscript = $state('');
	let uploadFile = $state<File | null>(null);
	let uploading = $state(false);

	const selectedVoice = $derived(selectedVoiceIdx >= 0 ? voices[selectedVoiceIdx] : null);

	function handleVoiceChange(e: Event) {
		selectedVoiceIdx = parseInt((e.target as HTMLSelectElement).value);
		ttsAudioSrc = '';
	}

	async function generateTTS() {
		if (!selectedVoice) return;
		ttsGenerating = true;
		ttsProgress = 0;
		ttsAudioSrc = '';

		const progressSource = new EventSource('/api/tts/progress');
		progressSource.onmessage = (e) => {
			try {
				const data = JSON.parse(e.data);
				if (data.progress >= 0) ttsProgress = data.progress;
				if (data.progress >= 1 || data.progress < 0) progressSource.close();
			} catch {}
		};
		progressSource.onerror = () => progressSource.close();

		try {
			const url = recapTTSURL(campaignId, selectedVoice) + `&_t=${Date.now()}`;
			const res = await fetch(url);
			if (res.ok) {
				const blob = await res.blob();
				ttsAudioSrc = URL.createObjectURL(blob);
			}
		} catch {}

		progressSource.close();
		ttsGenerating = false;
		ttsProgress = 0;
	}

	async function handleUpload() {
		if (!uploadName || !uploadFile) return;
		uploading = true;
		try {
			await uploadVoiceProfile(campaignId, uploadName, uploadFile, uploadTranscript);
			voices = await fetchRecapVoices(campaignId);
			uploadName = '';
			uploadTranscript = '';
			uploadFile = null;
			showUpload = false;
		} catch {}
		uploading = false;
	}

	async function handleDeleteProfile(profileId: number) {
		try {
			await deleteVoiceProfile(profileId);
			voices = await fetchRecapVoices(campaignId);
			selectedVoiceIdx = -1;
			ttsAudioSrc = '';
		} catch {}
	}

	const refAudioSrc = $derived(
		selectedVoice && !selectedVoice.is_custom
			? `/api/campaigns/${campaignId}/recap/ref?voice=${encodeURIComponent(selectedVoice.user_id)}&_t=${Date.now()}`
			: selectedVoice?.is_custom && selectedVoice.profile_id
				? `/api/voice-profiles/${selectedVoice.profile_id}/audio`
				: ''
	);

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
			recap = await regenerateRecap(campaignId, lastN, recapStyle);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to generate recap';
		} finally {
			generating = false;
		}
	}

	onMount(() => {
		loadRecap();
		fetchRecapVoices(campaignId).then(v => { voices = v; }).catch(() => {});
	});
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
				<label class="last-n-label">
					Style:
					<select class="style-select" bind:value={recapStyle}>
						<option value="default">Default</option>
						<option value="dramatic">Dramatic</option>
						<option value="casual">Casual</option>
						<option value="in-character">In-Character</option>
					</select>
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
				<label class="last-n-label">
					Style:
					<select class="style-select" bind:value={recapStyle}>
						<option value="default">Default</option>
						<option value="dramatic">Dramatic</option>
						<option value="casual">Casual</option>
						<option value="in-character">In-Character</option>
					</select>
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

		<div class="tts-section">
			<div class="tts-controls">
				<label class="tts-label">
					Listen with voice:
					<select class="tts-select" onchange={handleVoiceChange} value={selectedVoiceIdx}>
						<option value={-1}>-- Select a voice --</option>
						{#each voices as voice, idx}
							<option value={idx}>{voice.display_name}{voice.is_custom ? ' (custom)' : ''}</option>
						{/each}
					</select>
				</label>
				{#if selectedVoice}
					<button class="regenerate-btn" onclick={generateTTS} disabled={ttsGenerating}>
						{#if ttsGenerating}
							<span class="spinner"></span>
							Generating...
						{:else}
							Generate
						{/if}
					</button>
					{#if selectedVoice.is_custom && selectedVoice.profile_id}
						<button class="regenerate-btn" style="color: #f87171;" onclick={() => handleDeleteProfile(selectedVoice!.profile_id!)}>Delete</button>
					{/if}
				{/if}
				<button class="regenerate-btn" onclick={() => showUpload = !showUpload}>
					{showUpload ? 'Cancel' : 'Upload Voice'}
				</button>
			</div>
			{#if showUpload}
				<div class="upload-form">
					<input class="upload-input" type="text" placeholder="Name (e.g. Matt Mercer)" bind:value={uploadName} />
					<input class="upload-input" type="text" placeholder="Transcript of audio (optional, improves quality)" bind:value={uploadTranscript} />
					<input type="file" accept="audio/*" onchange={(e) => { uploadFile = (e.target as HTMLInputElement).files?.[0] ?? null; }} />
					<button class="regenerate-btn" onclick={handleUpload} disabled={uploading || !uploadName || !uploadFile}>
						{uploading ? 'Uploading...' : 'Upload'}
					</button>
				</div>
			{/if}
				{#if ttsGenerating}
					<div class="tts-progress">
						<div class="tts-progress-header">
							<span>Generating audio...</span>
							<span class="tts-progress-pct">{Math.round(ttsProgress * 100)}%</span>
						</div>
						<div class="tts-progress-track">
							<div class="tts-progress-fill" style="width: {ttsProgress * 100}%"></div>
						</div>
					</div>
				{/if}
				{#if refAudioSrc}
					<p class="tts-label" style="margin-top: 0.75rem;">Reference clip:</p>
					<AudioPlayer src={refAudioSrc} />
				{/if}
				{#if ttsAudioSrc}
					<p class="tts-label" style="margin-top: 0.5rem;">Generated:</p>
					<AudioPlayer src={ttsAudioSrc} />
				{/if}
			</div>

		<div class="previously-on-section">
			<div class="previously-on-header">
				<h2 class="previously-on-title">Previously On...</h2>
				<button class="regenerate-btn" onclick={handleGeneratePreviouslyOn} disabled={prevOnLoading}>
					{#if prevOnLoading}
						<span class="spinner"></span>
						Generating...
					{:else}
						{previouslyOn ? 'Regenerate' : 'Generate'}
					{/if}
				</button>
			</div>
			{#if prevOnError}
				<div class="error-box">{prevOnError}</div>
			{/if}
			{#if previouslyOn}
				<div class="previously-on-body">
					{#each previouslyOn.text.split('\n\n') as paragraph}
						{#if paragraph.trim()}
							<p>{paragraph.trim()}</p>
						{/if}
					{/each}
				</div>
			{:else if !prevOnLoading}
				<p class="muted" style="font-size: 0.85rem;">Generate a dramatic "Previously on..." narration from the last session, designed to be read aloud.</p>
			{/if}
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

	.style-select {
		padding: 0.3rem 0.5rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		font-size: 0.85rem;
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

	.tts-section {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem 1.25rem;
		margin-bottom: 1.25rem;
	}
	.tts-controls {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	.tts-label {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--text-secondary);
		font-size: 0.85rem;
		font-weight: 500;
	}
	.tts-select {
		padding: 0.35rem 0.6rem;
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		font-size: 0.85rem;
	}

	.upload-form {
		margin-top: 0.75rem;
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: center;
		padding: 0.75rem;
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.upload-input {
		padding: 0.35rem 0.6rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		font-size: 0.85rem;
		flex: 1;
		min-width: 150px;
	}
	.upload-input::placeholder {
		color: var(--text-muted);
	}

	.tts-progress {
		margin-top: 0.75rem;
	}
	.tts-progress-header {
		display: flex;
		justify-content: space-between;
		font-size: 0.8rem;
		color: var(--text-muted);
		margin-bottom: 0.35rem;
	}
	.tts-progress-pct {
		color: var(--accent-gold);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.tts-progress-track {
		height: 6px;
		background: var(--bg-dark);
		border-radius: 3px;
		overflow: hidden;
	}
	.tts-progress-fill {
		height: 100%;
		background: var(--accent-gold);
		border-radius: 3px;
		transition: width 0.3s ease;
	}

	.previously-on-section {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem 1.5rem;
		margin-bottom: 1.25rem;
	}
	.previously-on-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.75rem;
	}
	.previously-on-title {
		color: var(--accent-gold);
		font-size: 1.1rem;
		font-weight: 600;
		margin: 0;
		font-style: italic;
	}
	.previously-on-body p {
		color: var(--text-primary);
		font-size: 1rem;
		line-height: 1.7;
		font-style: italic;
		margin-bottom: 1rem;
	}
	.previously-on-body p:last-child {
		margin-bottom: 0;
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
