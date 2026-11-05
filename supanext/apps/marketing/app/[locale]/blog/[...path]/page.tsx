import { PostContent } from "@blog/components/PostContent";
import { getPostBySlug, getPublishedPostPaths } from "@blog/lib/posts";
import { LocaleLink, localeRedirect } from "@i18n/routing";
import { getBaseUrl } from "@shared/lib/base-url";
import { getActivePathFromUrlParam } from "@shared/lib/content";
import Image from "next/image";
import { getTranslations, setRequestLocale } from "next-intl/server";

export function generateStaticParams() {
	const paths = getPublishedPostPaths();
	return paths.map((path) => ({ path: [path] }));
}

type Params = {
	path: string;
	locale: string;
};

export async function generateMetadata(props: { params: Promise<Params> }) {
	const { path, locale } = await props.params;
	const slug = getActivePathFromUrlParam(path);
	const post = await getPostBySlug(slug, { locale });

	return {
		title: post?.title,
		description: post?.excerpt,
		openGraph: {
			title: post?.title,
			description: post?.excerpt,
			images: post?.image
				? [
						post.image.startsWith("http")
							? post.image
							: new URL(post.image, getBaseUrl()).toString(),
					]
				: [],
		},
	};
}

export default async function BlogPostPage(props: { params: Promise<Params> }) {
	const { path, locale } = await props.params;
	setRequestLocale(locale);

	const t = await getTranslations({ locale, namespace: "blog" });

	const slug = getActivePathFromUrlParam(path);
	const post = await getPostBySlug(slug, { locale });

	if (!post) {
		return localeRedirect({ href: "/blog", locale });
	}

	const { title, date, authorName, authorImage, tags, image, body } = post;

	return (
		<div className="container py-16">
			<div className="">
				<div className="mb-12">
					<LocaleLink href="/blog">&larr; {t("back")}</LocaleLink>
				</div>

				<div className="max-w-2xl mx-auto text-center">
					<h1 className="font-bold text-4xl">{title}</h1>

					<div className="mt-4 flex items-center justify-center gap-6">
						{authorName && (
							<div className="flex items-center">
								{authorImage && (
									<div className="relative mr-2 size-8 overflow-hidden rounded-full">
										<Image
											src={authorImage}
											alt={authorName}
											fill
											sizes="96px"
											className="object-cover object-center"
										/>
									</div>
								)}
								<div>
									<p className="font-semibold text-sm opacity-50">
										{authorName}
									</p>
								</div>
							</div>
						)}

						<div className="mr-0">
							<p className="text-sm opacity-30">
								{Intl.DateTimeFormat("en-US").format(
									new Date(date),
								)}
							</p>
						</div>

						{tags && (
							<div className="flex flex-wrap gap-2">
								{tags.map((tag) => (
									<span
										key={tag}
										className="font-semibold text-primary text-xs uppercase tracking-wider"
									>
										#{tag}
									</span>
								))}
							</div>
						)}
					</div>
				</div>
			</div>

			{image && (
				<div className="relative mt-6 aspect-video overflow-hidden rounded-4xl bg-primary/10 p-4 lg:p-6">
					<Image
						src={image}
						alt={title}
						fill
						sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
						className="object-cover object-center rounded-xl"
					/>
				</div>
			)}

			<div className="pb-8">
				<PostContent content={body} />
			</div>
		</div>
	);
}
