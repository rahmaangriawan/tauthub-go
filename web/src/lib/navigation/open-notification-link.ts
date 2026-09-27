// Port src/lib/navigation/open-notification-link.ts (repo lama).
// Resolusi tautan notifikasi tanpa mengizinkan skema berbahaya (javascript:,
// http: non-lokal), lalu navigasi internal/eksternal setelah menandai dibaca.

export type NotificationNavigationTarget =
	| { kind: 'internal'; href: string }
	| { kind: 'external'; href: string };

export function notificationNavigationTarget(
	link: string | null | undefined,
	origin: string
): NotificationNavigationTarget | null {
	const value = link?.trim();
	if (!value) return null;

	const isPath = value.startsWith('/') || value.startsWith('?') || value.startsWith('#');
	const isHttpUrl = /^https?:\/\//i.test(value);
	if (!isPath && !isHttpUrl) return null;

	let url: URL;
	try {
		url = new URL(value, origin);
	} catch {
		return null;
	}

	if (url.origin === origin) {
		return { kind: 'internal', href: `${url.pathname}${url.search}${url.hash}` };
	}
	if (url.protocol === 'https:') {
		return { kind: 'external', href: url.href };
	}
	return null;
}

export async function openNotificationLink(options: {
	link: string | null | undefined;
	fallbackLink?: string | null;
	push: (href: string) => void | Promise<void>;
	markRead: () => Promise<unknown>;
	origin?: string;
	openExternal?: (href: string) => void;
}): Promise<NotificationNavigationTarget | null> {
	try {
		await options.markRead();
	} catch {
		// Pelacakan baca tidak boleh menghalangi tujuan notifikasi yang valid.
	}

	const origin = options.origin ?? window.location.origin;
	const target =
		notificationNavigationTarget(options.link, origin) ??
		notificationNavigationTarget(options.fallbackLink, origin);
	if (!target) return null;

	if (target.kind === 'internal') {
		await options.push(target.href);
	} else {
		(options.openExternal ?? ((href) => window.location.assign(href)))(target.href);
	}
	return target;
}
