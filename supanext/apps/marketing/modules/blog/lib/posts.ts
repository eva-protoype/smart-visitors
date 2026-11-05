import type { Post } from "@blog/types";
import { config as i18nConfig } from "@i18n/config";
import {
	getLocalizedDocumentWithFallback,
	getUniqueBasePaths,
} from "@shared/lib/content";
import { allPosts } from "content-collections";

const defaultLocale = i18nConfig.defaultLocale;

/**
 * Returns paths of all published posts for use in generateStaticParams.
 */
export function getPublishedPostPaths(): string[] {
	const paths = getUniqueBasePaths(allPosts);
	return paths.filter((path) => {
		const post = getLocalizedDocumentWithFallback(
			allPosts,
			path,
			defaultLocale,
			{ defaultLocale },
		);
		return post?.published === true;
	});
}

/**
 * Returns all posts for the given locale. Always includes default-language posts;
 * localized versions (e.g. .de.mdx) overwrite content only when they exist.
 */
export async function getAllPosts(locale?: string): Promise<Post[]> {
	const resolvedLocale = locale ?? defaultLocale;
	const paths = getUniqueBasePaths(allPosts);

	const posts = paths
		.map((path) =>
			getLocalizedDocumentWithFallback(allPosts, path, resolvedLocale, {
				defaultLocale,
			}),
		)
		.filter((post): post is NonNullable<typeof post> => post != null)
		.filter((post) => post.published);

	return Promise.resolve(
		posts.sort(
			(a, b) => new Date(b.date).getTime() - new Date(a.date).getTime(),
		),
	);
}

/**
 * Returns a post by slug for the given locale. Falls back to default-language
 * content when no localized version exists.
 */
export async function getPostBySlug(
	slug: string,
	options?: {
		locale?: string;
	},
): Promise<Post | null> {
	const resolvedLocale = options?.locale ?? defaultLocale;
	const post = getLocalizedDocumentWithFallback(
		allPosts,
		slug,
		resolvedLocale,
		{ defaultLocale },
	);

	return Promise.resolve(post ?? null);
}
