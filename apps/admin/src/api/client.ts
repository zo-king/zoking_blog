const runtimeApiBase = window.__ZOKING_ADMIN_CONFIG__?.apiBaseUrl?.trim();
export const apiBase = runtimeApiBase || import.meta.env.VITE_API_BASE_URL || "http://localhost:18080";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly code = "",
    public readonly details?: unknown,
    public readonly requestID = "",
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(path: string, options: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has("Content-Type") && options.body && !(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const method = (options.method || "GET").toUpperCase();
  if (token && !["GET", "HEAD", "OPTIONS"].includes(method)) headers.set("X-CSRF-Token", token);

  const response = await fetch(`${apiBase}${path}`, { ...options, headers, credentials: "include" });
  if (!response.ok) {
    const text = await response.text();
    try {
      const payload = JSON.parse(text) as { error?: { code?: string; message?: string; details?: unknown }; request_id?: string };
      throw new ApiError(
        response.status,
        payload.error?.message || `${response.status} ${response.statusText}`,
        payload.error?.code,
        payload.error?.details,
        payload.request_id,
      );
    } catch (error) {
      if (error instanceof ApiError) throw error;
      throw new ApiError(response.status, text || `${response.status} ${response.statusText}`);
    }
  }
  return response.json() as Promise<T>;
}
