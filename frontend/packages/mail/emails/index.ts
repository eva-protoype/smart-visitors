import { EmailVerification } from "./EmailVerification";
import { ForgotPassword } from "./ForgotPassword";
import { MagicLink } from "./MagicLink";
import { NewUser } from "./NewUser";
import { OrganizationInvitation } from "./OrganizationInvitation";

export const mailTemplates = {
	magicLink: MagicLink,
	forgotPassword: ForgotPassword,
	newUser: NewUser,
	organizationInvitation: OrganizationInvitation,
	emailVerification: EmailVerification,
} as const;
