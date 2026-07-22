import { create } from "zustand";

interface User {
  id: string;
  email: string;
  name: string;
  createdAt: number;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => void;
  checkAuth: () => Promise<void>;
}

const TOKEN_KEY = "wacalls.token";
const FETCH_TIMEOUT_MS = 10000;

const fetchWithTimeout = (url: string, init?: RequestInit, timeoutMs = FETCH_TIMEOUT_MS): Promise<Response> => {
  const controller = new AbortController();
  const timer = setTimeout(() => {
    console.error(`[AUTH] fetch timeout (${timeoutMs}ms): ${init?.method || "GET"} ${url}`);
    controller.abort();
  }, timeoutMs);
  return fetch(url, { ...init, signal: controller.signal }).finally(() => clearTimeout(timer));
};

export const useAuth = create<AuthState>((set, get) => ({
  user: null,
  token: localStorage.getItem(TOKEN_KEY),
  isLoading: false,

  login: async (email, password) => {
    set({ isLoading: true });
    try {
      console.log(`[AUTH] login: ${email}`);
      const res = await fetchWithTimeout("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: "login failed" }));
        throw new Error(err.error);
      }
      const { user, token } = await res.json();
      localStorage.setItem(TOKEN_KEY, token);
      console.log(`[AUTH] login OK, user=${user.email}`);
      set({ user, token, isLoading: false });
    } catch (e: any) {
      console.error(`[AUTH] login error:`, e);
      set({ isLoading: false });
      throw e;
    }
  },

  register: async (email, password, name) => {
    set({ isLoading: true });
    try {
      const res = await fetchWithTimeout("/api/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password, name }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: "register failed" }));
        throw new Error(err.error);
      }
      const { user, token } = await res.json();
      localStorage.setItem(TOKEN_KEY, token);
      set({ user, token, isLoading: false });
    } catch (e) {
      set({ isLoading: false });
      throw e;
    }
  },

  logout: () => {
    console.log(`[AUTH] logout`);
    localStorage.removeItem(TOKEN_KEY);
    set({ user: null, token: null });
  },

  checkAuth: async () => {
    const token = get().token;
    if (!token) {
      console.log(`[AUTH] checkAuth: no token, showing login`);
      return;
    }
    set({ isLoading: true });
    try {
      console.log(`[AUTH] checkAuth: validating token...`);
      const res = await fetchWithTimeout("/api/auth/me", {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        console.warn(`[AUTH] checkAuth: token invalid (${res.status})`);
        localStorage.removeItem(TOKEN_KEY);
        set({ user: null, token: null, isLoading: false });
        return;
      }
      const user = await res.json();
      console.log(`[AUTH] checkAuth: OK, user=${user.email}`);
      set({ user, isLoading: false });
    } catch (e: any) {
      console.error(`[AUTH] checkAuth error:`, e?.message || e);
      localStorage.removeItem(TOKEN_KEY);
      set({ user: null, token: null, isLoading: false });
    }
  },
}));

export const getAuthToken = (): string | null => localStorage.getItem(TOKEN_KEY);
