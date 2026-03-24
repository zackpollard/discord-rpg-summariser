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

// Auth types and functions

export interface AuthUser {
	id: string;
	username: string;
	avatar: string;
}

export async function fetchAuthMe(): Promise<AuthUser> {
	return request<AuthUser>('/api/auth/me');
}

export async function logout(): Promise<void> {
	await request<void>('/api/auth/logout', { method: 'POST' });
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

export async function deleteSession(sessionId: number): Promise<void> {
	await request<void>(`/api/sessions/${sessionId}`, { method: 'DELETE' });
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
	status: string;
	cause_of_death: string;
	parent_entity_id: number | null;
	created_at: string;
	updated_at: string;
}

export interface EntityParent {
	id: number;
	name: string;
}

export interface EntityChild {
	id: number;
	name: string;
}

export interface EntitySessionAppearance {
	session_id: number;
	started_at: string;
	mention_count: number;
}

export interface EntityReference {
	session_id: number;
	segment_id: number | null;
	start_time: number;
	context: string;
}

export interface EntityDetail extends Entity {
	notes: EntityNote[];
	relationships: EntityRelationshipDisplay[];
	sessions: EntitySessionAppearance[];
	references: EntityReference[];
	parent: EntityParent | null;
	children: EntityChild[];
}

export interface LocationNode {
	id: number;
	name: string;
	description: string;
	parent_id: number | null;
	children: LocationNode[];
	created_at: string;
	updated_at: string;
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

export async function fetchEntities(campaignId: number, params?: { type?: string; search?: string; status?: string }): Promise<Entity[]> {
	const searchParams = new URLSearchParams();
	if (params?.type) searchParams.set('type', params.type);
	if (params?.search) searchParams.set('search', params.search);
	if (params?.status) searchParams.set('status', params.status);
	const qs = searchParams.toString();
	return request<Entity[]>(`/api/campaigns/${campaignId}/entities${qs ? '?' + qs : ''}`);
}

export async function fetchEntity(id: number): Promise<EntityDetail> {
	return request<EntityDetail>(`/api/entities/${id}`);
}

export async function renameEntity(id: number, name: string): Promise<void> {
	await request<void>(`/api/entities/${id}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
}

export async function mergeEntity(keepId: number, mergeId: number): Promise<void> {
	await request<void>(`/api/entities/${keepId}/merge`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ merge_id: mergeId })
	});
}

export async function fetchLocationHierarchy(campaignId: number): Promise<LocationNode[]> {
	return request<LocationNode[]>(`/api/campaigns/${campaignId}/location-hierarchy`);
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

export async function regenerateRecap(campaignId: number, lastN?: number): Promise<CampaignRecap> {
	const params = new URLSearchParams();
	if (lastN !== undefined) params.set('last', String(lastN));
	const qs = params.toString();
	return request<CampaignRecap>(`/api/campaigns/${campaignId}/recap${qs ? '?' + qs : ''}`, {
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

export function sessionAudioURL(sessionId: number): string {
	return `/api/sessions/${sessionId}/audio`;
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

// Entity timeline types and functions

export interface EntityTimelineEntry {
	entity_id: number;
	entity_name: string;
	entity_type: string;
	first_seen: string;
	last_seen: string;
	session_count: number;
	total_mentions: number;
}

export async function fetchEntityTimeline(campaignId: number): Promise<EntityTimelineEntry[]> {
	return request<EntityTimelineEntry[]>(`/api/campaigns/${campaignId}/entity-timeline`);
}

// Campaign stats types and functions

export interface SpeakerStat {
	user_id: string;
	character_name: string;
	segment_count: number;
	word_count: number;
}

export interface TopEntity {
	name: string;
	type: string;
	mentions: number;
}

export interface CombatActorStat {
	actor: string;
	actions: number;
	total_damage: number;
}

export interface SessionTimelineStat {
	session_id: number;
	started_at: string;
	duration_min: number;
	segment_count: number;
	word_count: number;
}

export interface CampaignStats {
	total_sessions: number;
	total_duration_min: number;
	avg_duration_min: number;
	total_segments: number;
	total_words: number;
	speaker_stats: SpeakerStat[];
	entity_counts: Record<string, number>;
	top_entities: TopEntity[];
	total_quests: number;
	active_quests: number;
	completed_quests: number;
	failed_quests: number;
	total_encounters: number;
	total_actions: number;
	total_damage: number;
	combat_actor_stats: CombatActorStat[];
	session_timeline: SessionTimelineStat[];
	npc_status_counts: Record<string, number>;
}

export async function fetchCampaignStats(campaignId: number): Promise<CampaignStats> {
	return request<CampaignStats>(`/api/campaigns/${campaignId}/stats`);
}

// PDF campaign book

export function campaignPDFURL(campaignId: number): string {
	return `/api/campaigns/${campaignId}/pdf`;
}
