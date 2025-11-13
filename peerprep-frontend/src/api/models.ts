const AI_BASE = import.meta.env.VITE_AI_BASE_URL ?? 'http://localhost:8086';

function aiUrl(endpoint: string): string {
  let base = AI_BASE.replace(/\/+$/, '');
  const path = endpoint.replace(/^\/+/, '');

  // If base already includes /api/v1/ai, just append endpoint
  if (base.endsWith('/api/v1/ai')) {
    return `${base}/${path}`;
  }

  // If base includes /api/v1 but not /ai, append /ai
  if (base.endsWith('/api/v1')) {
    return `${base}/ai/${path}`;
  }

  // Otherwise, append full path /api/v1/ai
  return `${base}/api/v1/ai/${path}`;
}

export interface ModelVersion {
  ID: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: string | null;
  version_name: string;
  base_model: string;
  training_job_id: string;
  training_data_size: number;
  is_active: boolean;
  traffic_weight: number;
  activated_at?: string | null;
  deactivated_at?: string | null;
}

export interface FeedbackStats {
  total_feedback: number;
  positive_feedback: number;
  negative_feedback: number;
  positive_rate: number;
}

export interface ModelStats {
  model: ModelVersion;
  feedback: FeedbackStats;
}

export async function listModels(activeOnly = false): Promise<{ ok: boolean; info: ModelVersion[] }> {
  const query = activeOnly ? '?active=true' : '';
  const res = await fetch(aiUrl(`models${query}`));
  if (!res.ok) {
    throw new Error(`Failed to fetch models: ${res.status}`);
  }
  return res.json();
}

export async function getModelStats(modelId: number): Promise<{ ok: boolean; info: ModelStats }> {
  const res = await fetch(aiUrl(`models/${modelId}/stats`));
  if (!res.ok) {
    throw new Error(`Failed to fetch model stats: ${res.status}`);
  }
  return res.json();
}

export async function updateTrafficWeight(modelId: number, trafficWeight: number): Promise<void> {
  const res = await fetch(aiUrl(`models/${modelId}/traffic`), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ traffic_weight: trafficWeight }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err?.info ?? `Failed to update traffic weight: ${res.status}`);
  }
}

export async function deactivateModel(modelId: number): Promise<void> {
  const res = await fetch(aiUrl(`models/${modelId}/deactivate`), {
    method: 'PUT',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err?.info ?? `Failed to deactivate model: ${res.status}`);
  }
}
