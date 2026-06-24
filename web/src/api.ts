/**
 * tsk REST client — typed wrapper over the JSON API exposed by `tsk serve`.
 *
 * Every method is tiny and explicit. We intentionally avoid request-batching
 * or caching: the server already re-reads .tsk.md per request, and the UI
 * is small enough that one fetch per action is fine.
 */

export type Priority = "low" | "medium" | "high" | "urgent";

export interface Task {
  id: number;
  title: string;
  done: boolean;
  priority: Priority;
  due?: string; // YYYY-MM-DD; missing or "" = no due date
  tags: string[];
  notes?: string;
  created?: string; // RFC3339
  completed?: string;
  depends_on?: number[]; // F26: ids this task is blocked by
}

export interface TaskListResponse {
  file: string;
  tasks: Task[];
}

export interface Stats {
  total: number;
  done: number;
  undone: number;
  overdue: number;
  today: number;
  completion: number;
  streak: number;
  top_tags: Array<{ tag: string; count: number }>;
}

export interface ParseDate {
  ok: boolean;
  input: string;
  date?: string; // YYYY-MM-DD when ok
  weekday?: string; // "Mon"
  pretty?: string; // "Sat, Jul 4 2026"
  relative?: string; // "in 10d"
  error?: string;
}

export interface TaskInput {
  title: string;
  priority?: Priority;
  due?: string;
  tags?: string[];
  notes?: string;
}

export interface TaskPatch {
  title?: string;
  priority?: Priority;
  due?: string;
  tags?: string[];
  notes?: string;
  done?: boolean;
  depends_on?: number[]; // F26
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, headers: {} };
  if (body !== undefined) {
    init.body = JSON.stringify(body);
    (init.headers as Record<string, string>)["Content-Type"] = "application/json";
  }
  const res = await fetch(path, init);
  const text = await res.text();
  let parsed: unknown = undefined;
  if (text.length > 0) {
    try {
      parsed = JSON.parse(text);
    } catch {
      // Non-JSON body — fall through; we'll surface raw text on error.
    }
  }
  if (!res.ok) {
    const msg =
      (parsed as { error?: string } | undefined)?.error ??
      text ??
      `HTTP ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return parsed as T;
}

export const api = {
  listTasks: () => request<TaskListResponse>("GET", "/api/tasks"),
  getTask: (id: number) => request<Task>("GET", `/api/tasks/${id}`),
  createTask: (input: TaskInput) => request<Task>("POST", "/api/tasks", input),
  patchTask: (id: number, patch: TaskPatch) =>
    request<Task>("PATCH", `/api/tasks/${id}`, patch),
  toggleTask: (id: number) => request<Task>("POST", `/api/tasks/${id}/toggle`),
  moveTask: (id: number, before: number) =>
    request<TaskListResponse>("POST", `/api/tasks/${id}/move`, { before }),
  deleteTask: (id: number) =>
    request<{ ok: boolean; id: number }>("DELETE", `/api/tasks/${id}`),
  stats: () => request<Stats>("GET", "/api/stats"),
  parseDate: (q: string) =>
    request<ParseDate>("GET", `/api/parse-date?q=${encodeURIComponent(q)}`),
  health: () => request<{ ok: boolean; file: string; now: string }>("GET", "/api/health"),
};
