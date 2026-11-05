import { expect, test } from "@playwright/test";

test.describe("home page", () => {
	test("should load", async ({ page }) => {
		await page.goto("/");

		await expect(
			page.getByRole("heading", {
				name: "Your revolutionary SaaS built with Next.js",
			}),
		).toBeVisible();
	});
});
