import { PostListItem } from "@blog/components/PostListItem";
import { getAllPosts } from "@blog/lib/posts";
import { getTranslations, setRequestLocale } from "next-intl/server";

export async function generateMetadata(props: {
	params: Promise<{ locale: string }>;
}) {
	const { locale } = await props.params;
	const t = await getTranslations({ locale, namespace: "blog" });
	return {
		title: t("title"),
	};
}

export default async function BlogListPage(props: {
	params: Promise<{ locale: string }>;
}) {
	const { locale } = await props.params;
	setRequestLocale(locale);

	const t = await getTranslations({ locale, namespace: "blog" });
	const posts = await getAllPosts(locale);

	return (
		<div className="container max-w-6xl py-16">
			<div className="mb-12 pt-8 text-center">
				<h1 className="mb-2 font-bold text-5xl">{t("title")}</h1>
				<p className="text-lg opacity-50">{t("description")}</p>
			</div>

			<div className="grid gap-8 md:grid-cols-2">
				{posts.map((post) => (
					<PostListItem post={post} key={post.path} />
				))}
			</div>
		</div>
	);
}
