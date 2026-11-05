import slugify from "slugify";

export type ContentStructureItem = {
	label: string;
	path: string;
	children: ContentStructureItem[];
	isPage: boolean;
};

export function getActivePathFromUrlParam(path: string | string[]) {
	return Array.isArray(path) ? path.join("/") : path || "";
}

/**
 * Resolves a document for a given path and locale.
 * - Prefers exact locale match (e.g. first-post.de.mdx for locale "de")
 * - Falls back to default locale (e.g. first-post.mdx) when no localized version exists
 * - Base files without locale suffix are always included as default-language content
 */
export function getLocalizedDocumentWithFallback<
	T extends { path: string; locale: string },
>(
	documents: T[],
	path: string,
	locale: string,
	options?: { defaultLocale?: string },
): T | undefined {
	const defaultLocale = options?.defaultLocale ?? "en";

	const matches = documents.filter((doc) => doc.path === path);
	if (matches.length === 0) {
		return undefined;
	}

	const priority = (doc: T) => {
		if (doc.locale === locale) {
			return 0;
		}
		if (doc.locale === defaultLocale) {
			return 1;
		}
		return 2;
	};

	return matches.sort((a, b) => priority(a) - priority(b))[0];
}

/**
 * Returns unique base paths from documents. Each path represents one content item;
 * localized variants (e.g. .de.mdx) share the same base path as the default file.
 */
export function getUniqueBasePaths<T extends { path: string }>(
	documents: T[],
): string[] {
	return [...new Set(documents.map((doc) => doc.path))];
}

export function slugifyHeadline(headline: string) {
	return slugify(headline, {
		lower: true,
		replacement: "-",
		trim: true,
		strict: true,
		remove: /[*+~.()'"!:@]/g,
	});
}
