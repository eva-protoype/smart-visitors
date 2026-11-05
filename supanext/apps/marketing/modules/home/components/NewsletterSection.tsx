"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Alert, AlertDescription, AlertTitle } from "@repo/ui/components/alert";
import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { CheckCircleIcon, KeyIcon } from "lucide-react";

import { useTranslations } from "next-intl";
import { useForm } from "react-hook-form";
import * as z from "zod";

const formSchema = z.object({
	email: z.email(),
});

export function NewsletterSection() {
	const t = useTranslations();

	const form = useForm({
		resolver: zodResolver(formSchema),
	});

	const onSubmit = form.handleSubmit(async ({ email }) => {
		try {
			// TODO: Insert your newsletter signup logic here to integrate with your CRM or email service
			console.log("Submitting newsletter signup for email:", email);
			await new Promise((resolve) => setTimeout(resolve, 1000));
		} catch {
			form.setError("email", {
				message: t("newsletter.hints.error.message"),
			});
		}
	});

	return (
		<section className="py-12 lg:py-16 bg-muted">
			<div className="container max-w-3xl mx-auto">
				<div className="mb-8 text-center">
					<KeyIcon className="mx-auto mb-3 size-10 text-primary" />
					<h1 className="font-medium text-lg md:text-xl lg:text-2xl xl:text-3xl leading-tighter text-foreground">
						{t("newsletter.title")}
					</h1>
					<p className="mt-2 text-foreground/60 text-sm sm:text-base">
						{t("newsletter.subtitle")}
					</p>
				</div>

				<div className="mx-auto max-w-lg flex flex-col items-center">
					{form.formState.isSubmitSuccessful ? (
						<Alert variant="success">
							<CheckCircleIcon />
							<AlertTitle>
								{t("newsletter.hints.success.title")}
							</AlertTitle>
							<AlertDescription>
								{t("newsletter.hints.success.message")}
							</AlertDescription>
						</Alert>
					) : (
						<form
							onSubmit={onSubmit}
							className="w-full max-w-md mx-auto"
						>
							<div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-center gap-2">
								<Input
									type="email"
									required
									placeholder={t("newsletter.email")}
									className="rounded-full"
									{...form.register("email")}
								/>

								<Button
									type="submit"
									variant="primary"
									loading={form.formState.isSubmitting}
								>
									{t("newsletter.submit")}
								</Button>
							</div>
							{form.formState.errors.email && (
								<p className="mt-1 text-destructive text-xs">
									{form.formState.errors.email.message}
								</p>
							)}
						</form>
					)}
				</div>
			</div>
		</section>
	);
}
