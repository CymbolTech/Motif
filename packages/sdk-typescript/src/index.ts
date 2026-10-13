export type EventRequest = {
  event_id: string;
  event_type: string;
  user_id?: string;
  occurred_at?: string;
  payload: Record<string, unknown>;
};

export type EventReceipt = {
  event_id: number;
  external_event_id: string;
  status: string;
};

export type PointBalance = {
  currency_id: string;
  currency: string;
  name: string;
  unit_scale: number;
  balance: number;
  account_id?: number;
};

export type UserPoints = { user_id: string; balances: PointBalance[] };

export type UserProfile = {
  user_id: string;
  event_count: number;
  balances: PointBalance[];
  missions: unknown[];
  achievements: unknown[];
  progression: unknown;
};

export type RedemptionRequest = {
  external_user_id: string;
  idempotency_key: string;
};

export type Redemption = {
  id: string;
  reward_id: string;
  external_user_id: string;
  status: string;
  idempotency_key: string;
  provider_reference?: string;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
};

type ErrorPayload = { error?: { code?: string; message?: string } };

export class MotifError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'MotifError';
    this.status = status;
    this.code = code;
  }
}

export type MotifClientOptions = {
  baseUrl: string;
  apiKey: string;
  fetch?: typeof globalThis.fetch;
};

export class MotifClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;
  private readonly fetcher: typeof globalThis.fetch;

  constructor(options: MotifClientOptions) {
    if (!options.baseUrl.trim()) throw new Error('baseUrl is required');
    if (!options.apiKey.trim()) throw new Error('apiKey is required');
    this.baseUrl = options.baseUrl.replace(/\/+$/, '');
    this.apiKey = options.apiKey;
    this.fetcher = options.fetch ?? globalThis.fetch;
    if (!this.fetcher) throw new Error('Fetch API is unavailable; pass fetch explicitly');
  }

  submitEvent(request: EventRequest): Promise<EventReceipt> {
    return this.request<EventReceipt>('/v1/events', { method: 'POST', body: request });
  }

  getEventStatus(externalEventId: string): Promise<EventReceipt> {
    return this.request<EventReceipt>(`/v1/events/${encodeURIComponent(externalEventId)}`);
  }

  getUserProfile(externalUserId: string): Promise<UserProfile> {
    return this.request<UserProfile>(`/v1/users/${encodeURIComponent(externalUserId)}/profile`);
  }

  getUserPoints(externalUserId: string): Promise<UserPoints> {
    return this.request<UserPoints>(`/v1/users/${encodeURIComponent(externalUserId)}/points`);
  }

  redeemReward(rewardId: string, request: RedemptionRequest): Promise<Redemption> {
    return this.request<Redemption>(`/v1/rewards/${encodeURIComponent(rewardId)}/redemptions`, { method: 'POST', body: request });
  }

  getRedemption(redemptionId: string): Promise<Redemption> {
    return this.request<Redemption>(`/v1/redemptions/${encodeURIComponent(redemptionId)}`);
  }

  retryRedemption(redemptionId: string): Promise<Redemption> {
    return this.request<Redemption>(`/v1/redemptions/${encodeURIComponent(redemptionId)}/retry`, { method: 'POST' });
  }

  private async request<T>(path: string, options: { method?: string; body?: unknown } = {}): Promise<T> {
    const response = await this.fetcher(`${this.baseUrl}${path}`, {
      method: options.method ?? 'GET',
      headers: {
        Accept: 'application/json',
        'X-Motif-API-Key': this.apiKey,
        ...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
    const payload = (await response.json().catch(() => ({}))) as T & ErrorPayload;
    if (!response.ok) {
      throw new MotifError(payload.error?.message ?? `MotifEngine request failed with ${response.status}`, response.status, payload.error?.code);
    }
    return payload as T;
  }
}

export default MotifClient;
