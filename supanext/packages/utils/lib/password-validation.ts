import { z } from "zod";

/**
 * Strong password validation schema that enforces:
 * - Minimum 8 characters
 * - At least one uppercase letter
 * - At least one lowercase letter
 * - At least one number
 * - At least one special character
 * - Allows Unicode characters and whitespace
 */
export const passwordSchema = z
	.string()
	.min(8, "Password must be at least 8 characters long")
	.max(255, "Password must be no more than 255 characters")
	.refine(
		(password) => password === password.trim(),
		"Password must not start or end with a space",
	)
	.refine(
		(password) => /[A-Z]/.test(password),
		"Password must contain at least one uppercase letter",
	)
	.refine(
		(password) => /[a-z]/.test(password),
		"Password must contain at least one lowercase letter",
	)
	.refine(
		(password) => /\d/.test(password),
		"Password must contain at least one number",
	)
	.refine(
		(password) => /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?~`]/.test(password),
		"Password must contain at least one special character",
	);
