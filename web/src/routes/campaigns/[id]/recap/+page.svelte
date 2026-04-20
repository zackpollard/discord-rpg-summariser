<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { fetchRecap, regenerateRecap, fetchPreviouslyOn, campaignPDFURL, fetchRecapVoices, recapTTSURL, fetchCachedTTS, uploadVoiceProfile, deleteVoiceProfile, type CampaignRecap, type RecapVoice, type PreviouslyOnResult, type TTSCacheEntry } from '$lib/api';
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
			// Force regeneration since the user clicked the button.
			previouslyOn = await fetchPreviouslyOn(campaignId, true);
		} catch (e) {
			prevOnError = e instanceof Error ? e.message : 'Failed to generate';
		} finally {
			prevOnLoading = false;
		}
	}

	// TTS voice picker state.
	let ttsSource = $state<'recap' | 'previously-on'>('recap');
	let voices = $state<RecapVoice[]>([]);
	let selectedVoiceIdx = $state<number>(-1);
	let ttsAudioSrc = $state<string>('');
	let ttsGenerating = $state(false);
	let ttsProgress = $state(0);
	let ttsError = $state<string | null>(null);
	let activeEventSource = $state<EventSource | null>(null);

	// TTS cache state.
	let ttsCache = $state<TTSCacheEntry[]>([]);

	// Voice profile upload state.
	let showUpload = $state(false);
	let uploadName = $state('');
	let uploadTranscript = $state('');
	let uploadFile = $state<File | null>(null);
	let uploading = $state(false);

	const selectedVoice = $derived(selectedVoiceIdx >= 0 ? voices[selectedVoiceIdx] : null);

	// Build voice key matching the server format for cache lookup.
	const currentVoiceKey = $derived.by(() => {
		const v = selectedVoice;
		if (!v) return '';
		if (v.is_custom && v.profile_id) return `profile:${v.profile_id}`;
		return `user:${v.user_id}`;
	});

	const cachedEntry = $derived(
		ttsCache.find(e => e.source === ttsSource && e.voice_key === currentVoiceKey) ?? null
	);

	// Stable URL for cached audio (server serves the file directly).
	const cachedAudioURL = $derived.by(() => {
		if (!cachedEntry || !selectedVoice) return '';
		return recapTTSURL(campaignId, selectedVoice) + `&source=${ttsSource}`;
	});

	// Compute refAudioSrc reactively based on selectedVoiceIdx (not Date.now()).
	let refAudioSrc = $state('');
	$effect(() => {
		const voice = selectedVoiceIdx >= 0 ? voices[selectedVoiceIdx] : null;
		if (voice && !voice.is_custom) {
			refAudioSrc = `/api/campaigns/${campaignId}/recap/ref?voice=${encodeURIComponent(voice.user_id)}&_t=${Date.now()}`;
		} else if (voice?.is_custom && voice.profile_id) {
			refAudioSrc = `/api/voice-profiles/${voice.profile_id}/audio`;
		} else {
			refAudioSrc = '';
		}
	});

	function handleVoiceChange(e: Event) {
		selectedVoiceIdx = parseInt((e.target as HTMLSelectElement).value);
		revokeTtsBlob();
		ttsAudioSrc = '';
	}

	function revokeTtsBlob() {
		if (ttsAudioSrc && ttsAudioSrc.startsWith('blob:')) {
			URL.revokeObjectURL(ttsAudioSrc);
		}
	}

	async function cancelTTS() {
		try {
			await fetch('/api/tts/cancel', { method: 'POST' });
		} catch {}
		activeEventSource?.close();
		activeEventSource = null;
		ttsGenerating = false;
		ttsProgress = 0;
	}

	async function generateTTS() {
		if (!selectedVoice) return;
		ttsGenerating = true;
		ttsProgress = 0;
		ttsError = null;
		revokeTtsBlob();
		ttsAudioSrc = '';

		// Close any previous EventSource before opening a new one.
		activeEventSource?.close();
		const progressSource = new EventSource('/api/tts/progress');
		activeEventSource = progressSource;
		progressSource.onmessage = (e) => {
			try {
				const data = JSON.parse(e.data);
				if (data.progress >= 0) ttsProgress = data.progress;
				if (data.progress >= 1 || data.progress < 0) progressSource.close();
			} catch {}
		};
		progressSource.onerror = () => progressSource.close();

		try {
			const url = recapTTSURL(campaignId, selectedVoice) + `&source=${ttsSource}&regenerate=true&_t=${Date.now()}`;
			const res = await fetch(url);
			if (res.ok) {
				const blob = await res.blob();
				revokeTtsBlob();
				ttsAudioSrc = URL.createObjectURL(blob);
				// Refresh cache list so the entry shows up.
				ttsCache = await fetchCachedTTS(campaignId);
			} else {
				ttsError = `TTS generation failed (${res.status})`;
			}
		} catch (e) {
			ttsError = e instanceof Error ? e.message : 'TTS generation failed';
		}

		progressSource.close();
		activeEventSource = null;
		ttsGenerating = false;
		ttsProgress = 0;
	}

	async function handleUpload() {
		if (!uploadName || !uploadFile) return;
		uploading = true;
		ttsError = null;
		try {
			await uploadVoiceProfile(campaignId, uploadName, uploadFile, uploadTranscript);
			voices = await fetchRecapVoices(campaignId);
			uploadName = '';
			uploadTranscript = '';
			uploadFile = null;
			showUpload = false;
		} catch (e) {
			ttsError = e instanceof Error ? e.message : 'Upload failed';
		}
		uploading = false;
	}

	async function handleDeleteProfile(profileId: number) {
		ttsError = null;
		try {
			await deleteVoiceProfile(profileId);
			voices = await fetchRecapVoices(campaignId);
			selectedVoiceIdx = -1;
			revokeTtsBlob();
			ttsAudioSrc = '';
		} catch (e) {
			ttsError = e instanceof Error ? e.message : 'Delete failed';
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

	function resumeIfGenerating() {
		// Check if a TTS generation is already in progress (e.g. started
		// before navigating away). If so, show the progress bar and subscribe.
		const es = new EventSource('/api/tts/progress');
		es.onmessage = (e) => {
			try {
				const data = JSON.parse(e.data);
				if (data.progress >= 0 && data.progress < 1.0) {
					// Generation is active — set source/voice and subscribe.
					if (data.source) ttsSource = data.source;
					if (data.voice_key && voices.length > 0) {
						const idx = voices.findIndex(v => {
							if (v.is_custom && v.profile_id) return `profile:${v.profile_id}` === data.voice_key;
							return `user:${v.user_id}` === data.voice_key;
						});
						if (idx >= 0) selectedVoiceIdx = idx;
					}
					ttsGenerating = true;
					ttsProgress = data.progress;
					activeEventSource = es;
					es.onmessage = (e2) => {
						try {
							const d = JSON.parse(e2.data);
							if (d.progress >= 0) ttsProgress = d.progress;
							if (d.progress >= 1.0 || d.progress < 0) {
								es.close();
								activeEventSource = null;
								ttsGenerating = false;
								ttsProgress = 0;
								fetchCachedTTS(campaignId).then(c => { ttsCache = c; }).catch(() => {});
							}
						} catch {}
					};
				} else {
					es.close();
				}
			} catch {
				es.close();
			}
		};
		es.onerror = () => es.close();
	}

	onMount(async () => {
		await loadRecap();
		// Pre-populate previously-on from cached recap response.
		if (recap?.previously_on) {
			previouslyOn = { text: recap.previously_on };
		}
		// Load voices first so resumeIfGenerating can match the voice_key.
		try { voices = await fetchRecapVoices(campaignId); } catch {}
		fetchCachedTTS(campaignId).then(c => { ttsCache = c; }).catch(() => {});
		resumeIfGenerating();
	});

	onDestroy(() => {
		activeEventSource?.close();
		revokeTtsBlob();
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
		<div class="recap-topbar">
			{#if recap.recap_generated_at}
				<span class="recap-date">Last generated: {formatDate(recap.recap_generated_at)}</span>
			{/if}
			<a href={campaignPDFURL(campaignId)} class="pdf-btn" download>Download PDF</a>
		</div>

		<section class="section-card">
			<div class="section-header">
				<div>
					<h2 class="section-title">Previously On...</h2>
					<p class="section-desc">A dramatic narration of the last session, designed to be read aloud at the start of the next game.</p>
				</div>
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
			{/if}
		</section>

		<section class="section-card">
			<div class="section-header">
				<div>
					<h2 class="section-title">Campaign Recap</h2>
					<p class="section-desc">A narrative summary of the entire campaign so far.</p>
				</div>
				<div class="recap-actions-inline">
					<label class="last-n-label">
						Last N:
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
			<div class="recap-body">
				{#each recap.recap.split('\n\n') as paragraph}
					{#if paragraph.trim()}
						<p>{paragraph.trim()}</p>
					{/if}
				{/each}
			</div>
		</section>

		<section class="section-card">
			<div class="section-header">
				<div>
					<h2 class="section-title">Voice Narration</h2>
					<p class="section-desc">Generate a voice-cloned audio reading of the campaign recap{previouslyOn ? ' or "Previously On..." narration' : ''}.</p>
				</div>
			</div>
			<div class="tts-controls">
				<label class="tts-label">
					Source:
					<select class="tts-select" bind:value={ttsSource} disabled={ttsGenerating}>
						<option value="recap">Campaign Recap</option>
						<option value="previously-on">Previously On...</option>
					</select>
				</label>
				<label class="tts-label">
					Voice:
					<select class="tts-select" onchange={handleVoiceChange} value={selectedVoiceIdx} disabled={ttsGenerating}>
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
							{cachedEntry ? 'Regenerate Audio' : 'Generate Audio'}
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
			{#if ttsError}
				<div class="error-box" style="margin-top: 0.75rem;">{ttsError}</div>
			{/if}
			{#if ttsGenerating}
				<div class="tts-progress">
					<div class="tts-progress-header">
						<span>Generating audio...</span>
						<span class="tts-progress-pct">{Math.round(ttsProgress * 100)}%</span>
						<button class="cancel-btn" onclick={cancelTTS}>Cancel</button>
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
				<div class="tts-audio-row">
					<p class="tts-label">Generated:</p>
					<a href={ttsAudioSrc} download="recap-audio.wav" class="download-btn">Download</a>
				</div>
				<AudioPlayer src={ttsAudioSrc} />
			{:else if cachedAudioURL && cachedEntry}
				<div class="tts-audio-row">
					<p class="tts-label">Cached audio <span class="muted">(generated {formatDate(cachedEntry.generated_at)})</span>:</p>
					<a href={cachedAudioURL} download="{ttsSource}-audio.wav" class="download-btn">Download</a>
				</div>
				<AudioPlayer src={cachedAudioURL} />
			{/if}
		</section>
	{/if}
</div>

<style>
	.recap-page {
		max-width: 800px;
	}

	.recap-topbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.25rem;
	}
	.recap-date {
		color: var(--text-muted);
		font-size: 0.8rem;
	}

	.section-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem 1.5rem;
		margin-bottom: 1.25rem;
	}
	.section-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		margin-bottom: 1rem;
	}
	.section-title {
		color: var(--accent-gold);
		font-size: 1.1rem;
		font-weight: 600;
		margin: 0;
	}
	.section-desc {
		color: var(--text-muted);
		font-size: 0.8rem;
		margin: 0.25rem 0 0;
	}
	.recap-actions-inline {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-shrink: 0;
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
		align-items: center;
		gap: 0.5rem;
		font-size: 0.8rem;
		color: var(--text-muted);
		margin-bottom: 0.35rem;
	}
	.tts-progress-header span:first-child {
		flex: 1;
	}
	.cancel-btn {
		background: none;
		border: 1px solid var(--border);
		color: #f87171;
		padding: 0.15rem 0.5rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.75rem;
	}
	.cancel-btn:hover {
		background: rgba(248, 113, 113, 0.1);
		border-color: #f87171;
	}
	.tts-audio-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: 0.5rem;
	}
	.tts-audio-row .tts-label {
		margin: 0;
	}
	.download-btn {
		font-size: 0.75rem;
		padding: 0.15rem 0.5rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-secondary);
		text-decoration: none;
	}
	.download-btn:hover {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
		text-decoration: none;
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
