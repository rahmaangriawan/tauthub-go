/**
 * Port lib/i18n/provider.tsx — locale state Svelte 5.
 * Storage key "tauthub-locale" (parity), default "id".
 */
import { dictionaries, type Locale, type Dict } from './dictionaries';

export type { Locale };

const STORAGE_KEY = 'tauthub-locale';
// Cookie dipakai SSR: server tidak bisa membaca localStorage, jadi pilihan
// bahasa disimpan juga sebagai cookie agar halaman pertama sudah berbahasa
// yang benar (tanpa kilatan Bahasa Indonesia lalu berganti).
export const LOCALE_COOKIE = 'tauthub-locale';

let locale = $state<Locale>('id');

function validLocale(value: string | null | undefined): Locale | null {
	return value === 'id' || value === 'en' ? value : null;
}

/** Set locale dari nilai yang diketahui server (cookie) — dipanggil layout
 *  akar SEBELUM anak-anaknya dirender, baik saat SSR maupun hidrasi. */
export function initLocale(value: string | null | undefined) {
	const l = validLocale(value);
	if (l) locale = l;
	applyHtmlLang();
}

export function initLocaleFromStorage() {
	if (typeof window === 'undefined') return;
	let stored: Locale | null = null;
	try {
		stored = validLocale(localStorage.getItem(STORAGE_KEY));
	} catch {
		/* localStorage diblokir */
	}
	// Pengguna lama: pilihan tersimpan di localStorage, cookie belum ada —
	// adopsi nilainya lalu tulis cookie supaya refresh berikutnya SSR benar.
	const l = stored ?? readCookieLocale();
	if (l) {
		locale = l;
		writeCookie(l);
	}
	applyHtmlLang();
}

function readCookieLocale(): Locale | null {
	try {
		const m = document.cookie.match(new RegExp(`(?:^|; )${LOCALE_COOKIE}=([^;]+)`));
		return validLocale(m ? decodeURIComponent(m[1]) : null);
	} catch {
		return null;
	}
}

function writeCookie(l: Locale) {
	try {
		document.cookie = `${LOCALE_COOKIE}=${l}; path=/; max-age=31536000; SameSite=Lax`;
	} catch {
		/* cookie diblokir */
	}
}

export function setLocale(l: Locale) {
	locale = l;
	applyHtmlLang();
	try {
		localStorage.setItem(STORAGE_KEY, l);
	} catch {
		/* quota exceeded */
	}
	writeCookie(l);
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

/**
 * applyHtmlLang: samakan <html lang> dengan bahasa yang dipilih dari menu
 * header. SSR tidak tahu pilihan pengguna (disimpan di localStorage), jadi
 * nilai awal dari app.html disesuaikan di klien begitu locale diketahui.
 */
function applyHtmlLang() {
	if (typeof document === 'undefined') return;
	document.documentElement.lang = locale;
}
