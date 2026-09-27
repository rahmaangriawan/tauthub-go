/**
 * Port src/lib/swr-cache.ts (repo lama) — universal in-memory SWR cache:
 * retrieval 0ms saat navigasi client, background update non-destruktif,
 * dedupe lintas komponen. TTL default 60 detik (parity perilaku polling).
 */

interface CacheEntry<T> {
	data: T;
	timestamp: number;
	ttl: number;
}

const GLOBAL_SWR_CACHE = new Map<string, CacheEntry<unknown>>();
const DEFAULT_TTL_MS = 60_000; // 1 menit fresh TTL

export function getSwrCache<T>(key: string): T | undefined {
	const entry = GLOBAL_SWR_CACHE.get(key);
	if (!entry) return undefined;
	return entry.data as T;
}

export function setSwrCache<T>(key: string, data: T, ttlMs: number = DEFAULT_TTL_MS): void {
	GLOBAL_SWR_CACHE.set(key, { data, timestamp: Date.now(), ttl: ttlMs });
}

export function invalidateSwrCache(keyPrefix?: string): void {
	if (keyPrefix) {
		for (const key of Array.from(GLOBAL_SWR_CACHE.keys())) {
			if (key.startsWith(keyPrefix)) {
				GLOBAL_SWR_CACHE.delete(key);
			}
		}
	} else {
		GLOBAL_SWR_CACHE.clear();
	}
}

export interface SwrOptions<T> {
	initialData?: T | null;
	ttlMs?: number;
	enabled?: boolean;
	revalidateOnFocus?: boolean;
}

export interface SwrState<T> {
	readonly data: T | null;
	readonly loading: boolean;
	readonly error: Error | null;
	mutate: (newData?: T | ((prev: T | null) => T), revalidate?: boolean) => Promise<void>;
	refetch: () => Promise<void>;
}

/**
 * Port useSwrData ke Svelte 5 runes. `key` = getter reaktif (null = idle).
 * Error TIDAK menghapus cache valid (non-destructive, parity).
 */
export function swr<T>(
	keySource: () => string | null,
	fetcher: () => Promise<T>,
	options: SwrOptions<T> = {}
): SwrState<T> {
	const opts = {
		initialData: null as T | null,
		ttlMs: DEFAULT_TTL_MS,
		enabled: true,
		revalidateOnFocus: true,
		...options
	};

	let data = $state<T | null>(opts.initialData);
	let loading = $state(false);
	let error = $state<Error | null>(null);
	let currentKey: string | null = null;

	async function fetchData(showLoading: boolean): Promise<void> {
		const key = keySource();
		if (!key || !opts.enabled) return;
		if (showLoading && !getSwrCache(key)) {
			loading = true;
		}
		try {
			const result = await fetcher();
			setSwrCache(key, result, opts.ttlMs);
			if (key === currentKey) {
				data = result;
				error = null;
				loading = false;
			}
		} catch (err) {
			if (key === currentKey) {
				error = err instanceof Error ? err : new Error(String(err));
				loading = false;
				// Non-destructive: cache valid tidak dilenyapkan (parity).
			}
		}
	}

	$effect(() => {
		const key = keySource();
		if (!key || !opts.enabled) {
			currentKey = null;
			return;
		}
		currentKey = key;
		const cached = getSwrCache<T>(key);
		if (cached !== undefined) {
			data = cached;
			loading = false;
		} else {
			loading = true;
		}
		void fetchData(cached === undefined);
	});

	// Revalidate on window focus (parity).
	$effect(() => {
		if (!opts.revalidateOnFocus || !opts.enabled) return;
		const onFocus = () => {
			if (currentKey) void fetchData(false);
		};
		window.addEventListener('focus', onFocus);
		return () => window.removeEventListener('focus', onFocus);
	});

	async function mutate(
		newData?: T | ((prev: T | null) => T),
		revalidate: boolean = true
	): Promise<void> {
		const key = currentKey;
		if (!key) return;
		if (newData !== undefined) {
			const next = typeof newData === 'function' ? (newData as (prev: T | null) => T)(data) : newData;
			data = next;
			setSwrCache(key, next, opts.ttlMs);
		}
		if (revalidate) {
			await fetchData(false);
		}
	}

	return {
		get data() {
			return data;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		mutate,
		refetch: () => fetchData(true)
	};
}
