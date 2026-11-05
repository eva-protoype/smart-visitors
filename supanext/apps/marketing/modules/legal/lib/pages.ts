import { config as i18nConfig } from "@i18n/config";
import {
	getLocalizedDocumentWithFallback,
	getUniqueBasePaths,
} from "@shared/lib/content";
import type { LegalPage } from "content-collections";
import { allLegalPages } from "content-collections";

const defaultLocale = i18nConfig.defaultLocale;

/**
 * Returns paths of all legal pages for use in generateStaticParams.
 */
export function getAllLegalPagePaths(): string[] {
	return getUniqueBasePaths(allLegalPages);
}

/**
 * Returns all legal pages for the given locale. Always includes default-language
 * pages; localized versions (e.g. .de.md) overwrite content only when they exist.
 */
export async function getAllLegalPages(
	locale?: string,
): Promise<Array<Omit<LegalPage, "_meta">>> {
	const resolvedLocale = locale ?? defaultLocale;
	const paths = getUniqueBasePaths(allLegalPages);

	const pages = paths
		.map((path) =>
			getLocalizedDocumentWithFallback(
				allLegalPages,
				path,
				resolvedLocale,
				{ defaultLocale },
			),
		)
		.filter((page): page is NonNullable<typeof page> => page != null);

	return Promise.resolve(pages);
}

/**
 * Returns a legal page by path for the given locale. Falls back to default-language
 * content when no localized version exists.
 */
export async function getLegalPageByPath(
	path: string,
	options?: {
		locale?: string;
	},
): Promise<Omit<LegalPage, "_meta"> | null> {
	const resolvedLocale = options?.locale ?? defaultLocale;
	const page = getLocalizedDocumentWithFallback(
		allLegalPages,
		path,
		resolvedLocale,
		{ defaultLocale },
	);

	return Promise.resolve(page ?? null);
}
