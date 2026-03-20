export interface Session {
	id: number;
	guild_id: string;
	campaign_id: number;
	channel_id: string;
	started_at: string;
	ended_at: string | null;
	status: string;
	summary: string | null;
	key_events: string[];
	created_at: string;
}

export interface TranscriptSegment {
	id: number;
	session_id: number;
	user_id: string;
	display_name: string;
	character_name: string | null;
	start_time: number;
	end_time: number;
	text: string;
	created_at: string;
}

export interface GuildMember {
	user_id: string;
	username: string;
	display_name: string;
}

export interface CharacterMapping {
	user_id: string;
	guild_id: string;
	character_name: string;
	updated_at: string;
}

export interface Status {
	recording: boolean;
	active_session: Session | null;
}

class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
		this.name = 'ApiError';
	}
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(path, init);
	if (!res.ok) {
		let msg = res.statusText;
		try {
			const body = await res.json();
			if (body.error) msg = body.error;
		} catch {
			// ignore parse errors
		}
		throw new ApiError(res.status, msg);
	}
	if (res.status === 204) return undefined as T;
	return res.json();
}

export async function fetchSessions(limit = 20, offset = 0, campaignId?: number): Promise<Session[]> {
	let url = `/api/sessions?limit=${limit}&offset=${offset}`;
	if (campaignId !== undefined) url += `&campaign_id=${campaignId}`;
	return request<Session[]>(url);
}

export async function fetchSession(id: number): Promise<Session> {
	return request<Session>(`/api/sessions/${id}`);
}

export async function fetchTranscript(sessionId: number): Promise<TranscriptSegment[]> {
	return request<TranscriptSegment[]>(`/api/sessions/${sessionId}/transcript`);
}

export async function reprocessSession(sessionId: number, retranscribe: boolean = false): Promise<void> {
	await request<void>(`/api/sessions/${sessionId}/reprocess`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ retranscribe })
	});
}

export async function fetchCharacters(): Promise<CharacterMapping[]> {
	return request<CharacterMapping[]>('/api/characters');
}

export async function upsertCharacter(userId: string, guildId: string, name: string): Promise<void> {
	await request<void>('/api/characters', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ user_id: userId, guild_id: guildId, character_name: name })
	});
}

export async function deleteCharacter(userId: string): Promise<void> {
	await request<void>(`/api/characters/${userId}`, { method: 'DELETE' });
}

export async function fetchMembers(): Promise<GuildMember[]> {
	return request<GuildMember[]>('/api/members');
}

export async function fetchStatus(): Promise<Status> {
	return request<Status>('/api/status');
}

export interface VoiceUser {
	user_id: string;
	display_name: string;
	speaking: boolean;
	packet_count: number;
	last_packet_at: string;
}

export function subscribeVoiceActivity(
	onUpdate: (users: VoiceUser[]) => void,
	onError?: (err: Event) => void
): () => void {
	const source = new EventSource('/api/voice-activity');
	source.onmessage = (event) => {
		try {
			const users: VoiceUser[] = JSON.parse(event.data);
			onUpdate(users);
		} catch {
			// ignore parse errors
		}
	};
	if (onError) source.onerror = onError;
	return () => source.close();
}

export interface LiveTranscriptEvent {
	user_id: string;
	display_name: string;
	start_time: number;
	end_time: number;
	text: string;
	partial: boolean;
	chunk_seq: number;
}

export function subscribeLiveTranscript(
	onSegment: (event: LiveTranscriptEvent) => void,
	onError?: (err: Event) => void
): () => void {
	const source = new EventSource('/api/live-transcript');
	source.onmessage = (event) => {
		try {
			onSegment(JSON.parse(event.data));
		} catch { }
	};
	if (onError) source.onerror = onError;
	return () => source.close();
}

// Campaign types and functions

export interface Campaign {
	id: number;
	guild_id: string;
	name: string;
	description: string;
	is_active: boolean;
	dm_user_id: string | null;
	created_at: string;
	recap: string;
	recap_generated_at: string | null;
}

export interface Entity {
	id: number;
	campaign_id: number;
	name: string;
	type: string;
	description: string;
	created_at: string;
	updated_at: string;
}

export interface EntityDetail extends Entity {
	notes: EntityNote[];
	relationships: EntityRelationshipDisplay[];
}

export interface EntityNote {
	id: number;
	session_id: number;
	content: string;
	created_at: string;
}

export interface EntityRelationshipDisplay {
	id: number;
	source_id: number;
	source_name: string;
	target_id: number;
	target_name: string;
	relationship: string;
	description: string;
}

export async function fetchCampaigns(): Promise<Campaign[]> {
	return request<Campaign[]>('/api/campaigns');
}

export async function createCampaign(name: string, description: string): Promise<Campaign> {
	return request<Campaign>('/api/campaigns', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name, description })
	});
}

export async function setActiveCampaign(id: number): Promise<void> {
	await request<void>(`/api/campaigns/${id}/active`, { method: 'PUT' });
}

export async function fetchEntities(campaignId: number, params?: { type?: string; search?: string }): Promise<Entity[]> {
	const searchParams = new URLSearchParams();
	if (params?.type) searchParams.set('type', params.type);
	if (params?.search) searchParams.set('search', params.search);
	const qs = searchParams.toString();
	return request<Entity[]>(`/api/campaigns/${campaignId}/entities${qs ? '?' + qs : ''}`);
}

export async function fetchEntity(id: number): Promise<EntityDetail> {
	return request<EntityDetail>(`/api/entities/${id}`);
}

export async function mergeEntity(keepId: number, mergeId: number): Promise<void> {
	await request<void>(`/api/entities/${keepId}/merge`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ merge_id: mergeId })
	});
}

// Quest types and functions

export interface Quest {
	id: number;
	campaign_id: number;
	name: string;
	description: string;
	status: string;
	giver: string;
	created_at: string;
	updated_at: string;
}

export interface QuestDetail extends Quest {
	updates: QuestUpdate[];
}

export interface QuestUpdate {
	id: number;
	quest_id: number;
	session_id: number;
	content: string;
	new_status: string | null;
	created_at: string;
}

export interface TimelineEvent {
	type: string;
	timestamp: string;
	title: string;
	detail: string;
	session_id: number | null;
	entity_id: number | null;
	quest_id: number | null;
}

export interface LoreSource {
	type: string;
	id: number;
	name: string;
	content: string;
}

export interface LoreAnswer {
	answer: string;
	sources: LoreSource[];
}

export interface LoreSearchResult {
	entity_id: number;
	name: string;
	type: string;
	snippet: string;
	score: number;
}

export interface CampaignRecap {
	recap: string;
	recap_generated_at: string | null;
}

export async function fetchQuests(campaignId: number, status?: string): Promise<Quest[]> {
	const params = new URLSearchParams();
	if (status) params.set('status', status);
	const qs = params.toString();
	return request<Quest[]>(`/api/campaigns/${campaignId}/quests${qs ? '?' + qs : ''}`);
}

export async function fetchQuest(id: number): Promise<QuestDetail> {
	return request<QuestDetail>(`/api/quests/${id}`);
}

export async function fetchTimeline(campaignId: number, limit?: number, offset?: number): Promise<TimelineEvent[]> {
	const params = new URLSearchParams();
	if (limit !== undefined) params.set('limit', String(limit));
	if (offset !== undefined) params.set('offset', String(offset));
	const qs = params.toString();
	return request<TimelineEvent[]>(`/api/campaigns/${campaignId}/timeline${qs ? '?' + qs : ''}`);
}

export async function askLore(campaignId: number, question: string): Promise<LoreAnswer> {
	return request<LoreAnswer>(`/api/campaigns/${campaignId}/lore/ask`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ question })
	});
}

export async function searchLore(campaignId: number, query: string): Promise<LoreSearchResult[]> {
	return request<LoreSearchResult[]>(`/api/campaigns/${campaignId}/lore/search?q=${encodeURIComponent(query)}`);
}

export async function fetchRecap(campaignId: number): Promise<CampaignRecap> {
	return request<CampaignRecap>(`/api/campaigns/${campaignId}/recap`);
}

export async function regenerateRecap(campaignId: number): Promise<CampaignRecap> {
	return request<CampaignRecap>(`/api/campaigns/${campaignId}/recap`, {
		method: 'POST'
	});
}

// Transcript search types and functions

export interface TranscriptSearchResult {
	segment_id: number;
	session_id: number;
	user_id: string;
	display_name: string;
	character_name: string | null;
	start_time: number;
	end_time: number;
	text: string;
	headline: string;
	session_started_at: string;
}

export interface TranscriptSearchResponse {
	results: TranscriptSearchResult[];
	total: number;
	limit: number;
	offset: number;
}

export async function searchTranscripts(
	campaignId: number,
	query: string,
	limit = 20,
	offset = 0
): Promise<TranscriptSearchResponse> {
	const params = new URLSearchParams({ q: query, limit: String(limit), offset: String(offset) });
	return request<TranscriptSearchResponse>(
		`/api/campaigns/${campaignId}/transcript-search?${params.toString()}`
	);
}

// Combat types and functions

export interface CombatAction {
	id: number;
	actor: string;
	action_type: string;
	target: string;
	detail: string;
	damage: number | null;
	round: number | null;
	timestamp: number | null;
}

export interface CombatEncounter {
	id: number;
	session_id: number;
	name: string;
	start_time: number;
	end_time: number;
	summary: string;
	created_at: string;
	actions: CombatAction[];
}

export async function fetchSessionCombat(sessionId: number): Promise<CombatEncounter[]> {
	return request<CombatEncounter[]>(`/api/sessions/${sessionId}/combat`);
}

// Relationship graph types and functions

export interface GraphNode {
	id: number;
	name: string;
	type: string;
}

export interface GraphEdge {
	source: number;
	target: number;
	relationship: string;
	description: string;
}

export interface RelationshipGraphData {
	nodes: GraphNode[];
	edges: GraphEdge[];
}

export async function fetchRelationshipGraph(campaignId: number): Promise<RelationshipGraphData> {
	return request<RelationshipGraphData>(`/api/campaigns/${campaignId}/relationship-graph`);
}
