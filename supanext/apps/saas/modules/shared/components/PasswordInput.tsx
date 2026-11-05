"use client";

import { cn, Input } from "@repo/ui";
import {
	CircleCheckIcon,
	CircleXIcon,
	EyeIcon,
	EyeOffIcon,
	RefreshCw,
} from "lucide-react";
import { useTranslations } from "next-intl";
import React from "react";

// Allowed special characters from password schema
const SPECIAL_CHARS = "!@#$%^&*()_+-=[]{};':\"\\|,.<>/?~`";

function generateValidPassword(): string {
	const uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
	const lowercase = "abcdefghijklmnopqrstuvwxyz";
	const numbers = "0123456789";

	// Ensure we have at least one of each required character type
	let password = "";
	password += uppercase[Math.floor(Math.random() * uppercase.length)];
	password += lowercase[Math.floor(Math.random() * lowercase.length)];
	password += numbers[Math.floor(Math.random() * numbers.length)];
	password += SPECIAL_CHARS[Math.floor(Math.random() * SPECIAL_CHARS.length)];

	// Fill the rest randomly (minimum 8 characters total, so 4 more)
	const allChars = uppercase + lowercase + numbers + SPECIAL_CHARS;
	for (let i = password.length; i < 12; i++) {
		password += allChars[Math.floor(Math.random() * allChars.length)];
	}

	// Shuffle the password to avoid predictable pattern
	return password
		.split("")
		.sort(() => Math.random() - 0.5)
		.join("");
}

interface PasswordCriterion {
	labelKey: string;
	check: (password: string) => boolean;
}

const passwordCriteria: PasswordCriterion[] = [
	{
		labelKey: "minLength",
		check: (password) => password.length >= 8,
	},
	{
		labelKey: "upperAndLowercase",
		check: (password) => /[A-Z]/.test(password) && /[a-z]/.test(password),
	},
	{
		labelKey: "number",
		check: (password) => /[0-9]/.test(password),
	},
	{
		labelKey: "specialCharacter",
		check: (password) =>
			/[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?~`]/.test(password),
	},
];

export function PasswordInput({
	value,
	onChange,
	className,
	inputClassName,
	autoComplete,
	name = "password",
	showGenerateButton = false,
	showPasswordCriteria = false,
}: {
	value?: string;
	onChange: (value: string) => void;
	className?: string;
	inputClassName?: string;
	autoComplete?: string;
	name?: string;
	showGenerateButton?: boolean;
	showPasswordCriteria?: boolean;
}) {
	const t = useTranslations();
	const [showPassword, setShowPassword] = React.useState(false);

	const generateRandomPassword = () => {
		const password = generateValidPassword();
		onChange(password);
		setShowPassword(true);
	};

	const rightPadding = showGenerateButton ? "pr-20" : "pr-10";
	const password = value || "";

	return (
		<div className={cn("", className)}>
			<div className="relative">
				<Input
					type={showPassword ? "text" : "password"}
					className={cn(rightPadding, inputClassName)}
					value={value}
					onChange={(e) => onChange(e.target.value)}
					autoComplete={autoComplete}
					name={name}
				/>
				<div className="absolute inset-y-0 right-0 flex items-center pr-2">
					{showGenerateButton && (
						<button
							type="button"
							onClick={generateRandomPassword}
							className="flex items-center justify-center p-2 text-primary hover:text-primary/80 transition-colors cursor-pointer"
							title="Generate random password"
						>
							<RefreshCw className="size-4" />
						</button>
					)}
					<button
						type="button"
						onClick={() => setShowPassword(!showPassword)}
						className="flex items-center justify-center p-2 text-primary hover:text-primary/80 transition-colors cursor-pointer"
						title={showPassword ? "Hide password" : "Show password"}
					>
						{showPassword ? (
							<EyeOffIcon className="size-4" />
						) : (
							<EyeIcon className="size-4" />
						)}
					</button>
				</div>
			</div>

			{showPasswordCriteria && (
				<div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
					{passwordCriteria.map((criterion, index) => {
						const isMet = criterion.check(password);
						return (
							<div
								key={index}
								className="flex items-center gap-1"
							>
								{isMet ? (
									<CircleCheckIcon className="size-3.5 text-success shrink-0" />
								) : (
									<CircleXIcon className="size-3.5 text-foreground/40 shrink-0" />
								)}
								<span
									className={cn(
										"text-xs",
										isMet
											? "text-success font-normal"
											: "text-foreground/40 font-light",
									)}
								>
									{t(
										`common.passwordCriteria.${criterion.labelKey}`,
									)}
								</span>
							</div>
						);
					})}
				</div>
			)}
		</div>
	);
}
