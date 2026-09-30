/**
 * Port lib/i18n/provider.tsx — locale state Svelte 5.
 * Storage key "tauthub-locale" (parity), default "id".
 */
import { dictionaries, type Locale, type Dict } from './dictionaries';

export type { Locale };

const STORAGE_KEY = 'tauthub-locale';

let locale = $state<Locale>('id');

export function initLocaleFromStorage() {
	if (typeof window === 'undefined') return;
	const stored = localStorage.getItem(STORAGE_KEY);
	if (stored === 'id' || stored === 'en') locale = stored;
}

export function setLocale(l: Locale) {
	locale = l;
	try {
		localStorage.setItem(STORAGE_KEY, l);
	} catch {
		/* quota exceeded */
	}
}

export function getLocale(): Locale {
	return locale;
}

/**
 * t = kamus locale aktif (struktur nested, parity useI18n().t).
 *
 * Nilai dikembalikan lewat getter, BUKAN properti biasa: `locale` adalah
 * `$state`, jadi tiap akses harus membaca ulang agar pemakai yang menulis
 * `$derived(getI18n().t)` ikut reaktif saat bahasa diganti. (Destrukturisasi
 * `const { t } = getI18n()` akan membekukan snapshot — pakai `$derived`.)
 */
export function getI18n(): { locale: Locale; t: Dict; setLocale: (l: Locale) => void } {
	return {
		get locale() {
			return locale;
		},
		get t() {
			return dictionaries[locale] as Dict;
		},
		setLocale
	};
}
