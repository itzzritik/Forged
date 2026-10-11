import type { Metadata } from "next";
import { Geist, Geist_Mono, Google_Sans_Code, Google_Sans_Flex } from "next/font/google";
import { ThemeProvider } from "next-themes";
import { TooltipProvider } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import "./globals.css";

const geist = Geist({ subsets: ["latin"], variable: "--font-sans" });

const geistMono = Geist_Mono({
	variable: "--font-geist-mono",
	subsets: ["latin"],
});

// Declared here so dialog portals get the variables; only [data-shell] pages use them, so marketing never downloads them.
const sansFlex = Google_Sans_Flex({ display: "swap", preload: false, subsets: ["latin"], variable: "--font-ui" });
const sansCode = Google_Sans_Code({ display: "swap", preload: false, subsets: ["latin"], variable: "--font-code" });

export const metadata: Metadata = {
	title: "Forged",
	description: "SSH key management. Forge your keys, take them anywhere.",
};

export default function RootLayout({
	children,
}: Readonly<{
	children: React.ReactNode;
}>) {
	return (
		<html
			className={cn("h-full scroll-smooth antialiased", geistMono.variable, geist.variable, sansFlex.variable, sansCode.variable, "font-sans")}
			lang="en"
			suppressHydrationWarning
		>
			<body className="flex min-h-full flex-col">
				<ThemeProvider attribute="class" defaultTheme="dark" enableSystem>
					<TooltipProvider>{children}</TooltipProvider>
				</ThemeProvider>
			</body>
		</html>
	);
}
