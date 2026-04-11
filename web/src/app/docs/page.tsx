import type { Metadata } from "next";
import Link from "next/link";
import { ScrollReveal } from "@/components/client";
import { DocsTOC, type DocsTOCGroup } from "@/components/docs-toc";

export const metadata: Metadata = {
	title: "Documentation - Forged",
	description: "Installation, setup, and configuration guide for Forged SSH key manager.",
};

function Code({ children }: { children: string }) {
	return (
		<code className="inline-flex -translate-y-px items-center border border-[#27272a] bg-black px-1.5 py-0.5 align-middle font-mono text-[#ea580c] text-[13px] leading-none shadow-[4px_4px_0px_rgba(39,39,42,1)]">
			{children}
		</code>
	);
}

function CodeBlock({ title, children }: { title?: string; children: string }) {
	return (
		<div className="group relative my-8 flex flex-col overflow-hidden border border-[#27272a] bg-black">
			{/* Background internal glow */}
			<div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-[#ea580c]/5 to-transparent opacity-0 transition-opacity duration-500 group-hover:opacity-100" />

			{title && (
				<div className="flex h-12 items-center justify-between border-[#27272a] border-b bg-[#09090b] px-5">
					<div className="flex items-center gap-3">
						<span className="h-1.5 w-1.5 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">
							SYSTEM {/* // */}
							{title}
						</span>
					</div>
					<span className="select-none font-mono text-[#3f3f46] text-[10px] uppercase tracking-widest">READY</span>
				</div>
			)}
			<div className="relative flex">
				<div className="pointer-events-none hidden w-12 shrink-0 select-none flex-col items-end border-[#27272a] border-r bg-[#09090b] py-5 pr-4 font-mono text-[#3f3f46] text-[11px] sm:flex">
					{children.split("\n").map((_, i) => (
						<span className="leading-7" key={i}>
							{i + 1}
						</span>
					))}
				</div>
				<pre className="flex-1 overflow-x-auto p-5 font-mono text-[13px] text-white leading-7 sm:pl-6">
					<code>{children}</code>
				</pre>
			</div>
		</div>
	);
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
	return (
		<section className="mb-32 scroll-mt-32 border-[#27272a] border-t pt-12 first:border-0 first:pt-0" id={id}>
			<div className="mb-8 flex items-center gap-2.5">
				<span className="h-3 w-1.5 bg-[#ea580c]" />
				<h2 className="m-0 font-bold text-3xl text-white tracking-tight">{title}</h2>
			</div>
			<div className="space-y-6 text-[#a1a1aa] text-lg leading-relaxed">{children}</div>
		</section>
	);
}

const tocGroups: DocsTOCGroup[] = [
	{
		title: "[ 01 ] // Getting Started",
		items: [
			{ href: "#installation", label: "Installation" },
			{ href: "#setup", label: "Setup Workflow" },
			{ href: "#usage", label: "Execution" },
		],
	},
	{
		title: "[ 02 ] // Core Concepts",
		items: [
			{ href: "#key-management", label: "Entity Management" },
			{ href: "#host-matching", label: "Host Matching" },
			{ href: "#git-signing", label: "Git Signing" },
		],
	},
	{
		title: "[ 03 ] // Advanced Setup",
		items: [
			{ href: "#sync", label: "Cloud Sync" },
			{ href: "#configuration", label: "Configuration" },
			{ href: "#commands", label: "Commands Ref" },
		],
	},
];

export default function DocsPage() {
	return (
		<div className="relative min-h-screen overflow-clip bg-black text-[#a1a1aa]">
			{/* Brutalist Repeating Background */}
			<div
				className="pointer-events-none fixed inset-0 opacity-[0.03]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>
			<div className="pointer-events-none fixed inset-y-0 left-0 z-10 w-8 bg-gradient-to-r from-black to-transparent" />

			<nav className="fixed top-0 right-0 left-0 z-50 border-[#27272a] border-b bg-black/80 backdrop-blur-xl">
				<div className="flex h-14 w-full items-center justify-between px-6 lg:px-16">
					<Link className="group flex items-center gap-3" href="/">
						<div className="flex h-7 w-7 items-center justify-center border border-[#27272a] bg-black transition-colors group-hover:border-[#ea580c]">
							<svg
								aria-label="Forged logo"
								className="text-white transition-colors group-hover:text-[#ea580c]"
								fill="none"
								height="14"
								role="img"
								stroke="currentColor"
								strokeLinecap="round"
								strokeLinejoin="round"
								strokeWidth="1.5"
								viewBox="0 0 24 24"
								width="14"
							>
								<path d="M15 3h6v6" />
								<path d="M10 14L21 3" />
								<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
							</svg>
						</div>
						<span className="font-bold font-mono text-[13px] text-white uppercase tracking-[0.2em] transition-colors group-hover:text-[#ea580c]">Forged</span>
					</Link>
					<div className="flex items-center gap-8">
						<Link className="text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white" href="/security">
							Security
						</Link>
						<a className="text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white" href="https://github.com/itzzritik/forged">
							GitHub
						</a>
					</div>
				</div>
			</nav>

			<div className="relative z-10 mx-auto flex w-full max-w-[1400px] flex-col gap-20 px-6 pt-32 pb-32 lg:flex-row lg:px-16">
				<DocsTOC groups={tocGroups} />

				<main className="min-w-0 max-w-4xl flex-[1.5] pt-2 pb-8">
					<ScrollReveal className="mb-12">
						<div className="mb-6 flex items-center gap-2.5">
							<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Operations Reference</span>
						</div>
						<h1 className="mb-8 text-pretty font-bold text-[clamp(3.5rem,6vw,7rem)] text-white leading-[0.9] tracking-tighter">Documentation.</h1>
						<p className="max-w-2xl text-[#a1a1aa] text-lg leading-relaxed lg:text-xl">
							Strictly rigorous infrastructure guidelines spanning installation, network configuration, and operational commands for the Forged daemon.
						</p>
					</ScrollReveal>

					<Section id="installation" title="Installation">
						<p>Forged is distributed as a single ~13MB binary compiled purely in Go with zero external CGO dependencies.</p>
						<CodeBlock title="macOS">brew install forged</CodeBlock>
						<CodeBlock title="Linux / macOS (BASH)">{"curl -fsSL https://forged.ritik.me/install.sh | sh"}</CodeBlock>
						<CodeBlock title="Compile locally">{"git clone https://github.com/itzzritik/forged\ncd forged\njust build-cli\n./bin/forged setup"}</CodeBlock>
					</Section>

					<Section id="setup" title="Setup Workflow">
						<p>
							Execute the initialization wizard to construct the encrypted vault, ingest your raw plaintext SSH keys from <Code>~/.ssh</Code>, bind the
							local daemon executable system services, and modify <Code>~/.ssh/config</Code>.
						</p>
						<CodeBlock title="Terminal">forged setup</CodeBlock>

						{/* Warning Diagnostic Alert */}
						<div className="group relative my-10 overflow-hidden border border-[#27272a] bg-black p-6 shadow-[4px_4px_0px_#ea580c]">
							<div className="mb-4 flex items-center gap-3 border-[#27272a] border-b pb-4">
								<svg aria-label="Warning" fill="none" height="18" role="img" stroke="#ea580c" strokeWidth="2" viewBox="0 0 24 24" width="18">
									<path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
									<line x1="12" x2="12" y1="9" y2="13" />
									<line x1="12" x2="12.01" y1="17" y2="17" />
								</svg>
								<span className="font-bold font-mono text-[#ea580c] text-[11px] uppercase tracking-[0.2em]">System Diagnostic Warning</span>
							</div>
							<p className="m-0 text-sm text-white/90 leading-relaxed md:text-base">
								A mandatory master password ensures cryptographic safety over the database locally using <span className="text-[#ea580c]">Argon2id</span>{" "}
								derivation. Protect this passphrase strictly.
							</p>
						</div>

						<p className="text-white/70">
							The daemon auto-boots on desktop login automatically via launchctl/systemd binding patterns. No external supervision necessary.
						</p>
					</Section>

					<Section id="usage" title="Execution">
						<p>
							Once bootstrapped, your CLI effectively passes through the Forged agent protocol. Compatible across any standard clients observing{" "}
							<Code>SSH_AUTH_SOCK</Code>.
						</p>
						<CodeBlock title="Workflow">
							{'$ ssh myserver                     # Resolves automatically\n$ git commit -m "deploy v2"        # Automatically signed via SSH key'}
						</CodeBlock>
					</Section>

					<Section id="key-management" title="Entity Management">
						<p>Manage the lifecycle of keys directly inside the vault without ever touching the filesystem in plaintext.</p>
						<CodeBlock title="Management">
							{
								'$ forged generate my-key -c "me@host"    # Auto-generates Ed25519\n$ forged add work --file ~/.ssh/id_ed25519  # Ingest existing payload\n$ forged list                               # Global index status\n$ forged list --json                        # CI Pipeline indexing\n$ forged export my-key                      # Output stdout PK\n$ forged rename my-key github               # Modify identifier\n$ forged remove old-key                     # Hard delete entity'
							}
						</CodeBlock>
						<p className="mt-8 border-[#ea580c]/50 border-l pl-4">Migrate payloads from existing sources using ingestion protocols:</p>
						<CodeBlock title="Migration Protocol">
							{
								"$ forged migrate --from ssh          # Import id_rsa/id_ed25519 from ~/.ssh/\n$ forged migrate --from 1password    # Import via 1Password CLI interface\n$ forged migrate --from agent        # List keys in current ssh-agent (public only)"
							}
						</CodeBlock>
					</Section>

					<Section id="host-matching" title="Regex & Host Matching">
						<p>
							Enforce strict mappings computationally. Banish &quot;Too many authentication attempts&quot; failures entirely by binding specific keys
							exclusively to specific domains.
						</p>
						<CodeBlock title="Routing Configuration">
							{
								'$ forged host github "github.com" "*.github.com"\n$ forged host deploy "*.prod.company.com" "10.0.*"\n$ forged host api "~^api\\\\d+\\\\.example\\\\.com$"  # Regex via ~ prefix\n$ forged hosts                       # List all active host mappings\n$ forged unhost deploy "10.0.*"      # Remove a host mapping'
							}
						</CodeBlock>
						<p className="mt-8 font-mono text-sm text-white/50 uppercase tracking-widest">[ Manual overrides via local architecture ]</p>
						<p className="mt-4">
							Alternatively, you can manually define patterns inside your local <Code>~/.forged/config.toml</Code>:
						</p>
						<CodeBlock title="config.toml">
							{
								'[[hosts]]\nname = "GitHub"\nmatch = ["github.com", "*.github.com"]\nkey = "github"\ngit_signing = true\n\n[[hosts]]\nname = "Production"\nmatch = ["*.prod.company.com", "10.0.*"]\nkey = "deploy"'
							}
						</CodeBlock>
					</Section>

					<Section id="git-signing" title="Signature Verification">
						<p>
							Enable rigorous provenance tracing by utilizing SSH signatures instead of traditional GPG protocols. The <Code>signing</Code> command
							configures your global Git settings automatically.
						</p>
						<CodeBlock title="Terminal">
							{
								"$ forged signing                     # Interactive key selector\n$ forged signing my-key              # Assign specific key for signing\n$ forged signing --off               # Disable Git commit signing"
							}
						</CodeBlock>
						<p className="mt-8 font-mono text-sm text-white/50 uppercase tracking-widest">[ Equivalent manual configuration ]</p>
						<p className="mt-4">
							Under the hood, this writes the following to your global <Code>~/.gitconfig</Code>:
						</p>
						<CodeBlock title="~/.gitconfig">
							{
								'[user]\n    signingkey = ssh-ed25519 AAAA...\n[gpg]\n    format = ssh\n[gpg "ssh"]\n    program = /path/to/forged-sign\n[commit]\n    gpgsign = true'
							}
						</CodeBlock>
					</Section>

					<Section id="sync" title="Multi-node Sync">
						<p>Operates over an isolated Blob infrastructure ensuring true zero-knowledge properties across device synchronization matrices.</p>
						<CodeBlock title="Sync Pipeline">
							{
								"$ forged login                # Init OAuth tokenization payload\n$ forged sync                 # Propagate full vault state\n$ forged sync status          # Monitor sync pipeline operations\n$ forged logout               # Scrub auth caches thoroughly"
							}
						</CodeBlock>
					</Section>

					<Section id="configuration" title="Core Configurations">
						<ul className="mb-8 space-y-4 border border-[#27272a] bg-black p-6 font-mono text-sm shadow-[4px_4px_0px_rgba(39,39,42,1)]">
							<li className="flex items-center gap-4">
								<span className="h-2 w-2 shrink-0 rounded-full bg-[#ea580c]" />
								<span className="min-w-[70px] text-[#a1a1aa]">macOS:</span> <span className="text-white">~/.forged/config.toml</span>
							</li>
							<li className="flex items-center gap-4">
								<span className="h-2 w-2 shrink-0 rounded-full bg-[#ea580c]" />
								<span className="min-w-[70px] text-[#a1a1aa]">Linux:</span> <span className="text-white">~/.config/forged/config.toml</span>
							</li>
						</ul>
						<CodeBlock title="config.toml">{'[agent]\nsocket = "~/.forged/agent.sock"\nlog_level = "info"\n\n[sync]\nenabled = false'}</CodeBlock>
					</Section>

					<Section id="commands" title="Unified Call Stack">
						<CodeBlock title="CLI Reference">
							{
								"# Lifecycle\nforged setup                     Bootstrap vault and daemon\nforged start / stop              Manage daemon service\nforged status                    Show daemon and key info\nforged doctor                    Diagnose common issues\nforged doctor --fix              Diagnose and auto-fix issues\nforged version                   Print version info\n\n# Keys\nforged generate [name]           Generate new Ed25519 key\nforged add <name> --file <path>  Import existing key\nforged list                      List all keys\nforged remove <name>             Delete a key\nforged export <name>             Output public key\nforged rename <old> <new>        Rename a key\n\n# Host Routing\nforged host <key> <patterns>     Map key to host patterns\nforged hosts                     List all host mappings\nforged unhost <key> <pattern>    Remove a host mapping\n\n# Git Signing\nforged signing [key]             Configure commit signing\nforged signing --off             Disable commit signing\n\n# Cloud Sync\nforged login                     Authenticate via browser\nforged sync                      Sync vault to cloud\nforged sync status               Show sync state\nforged logout                    Clear credentials\n\n# Maintenance\nforged enable / disable          Toggle SSH agent integration\nforged change-password           Change master password\nforged migrate --from <source>   Import from ssh/1password/agent\nforged benchmark                 Test Argon2id performance\nforged logs                      Tail daemon logs"
							}
						</CodeBlock>
					</Section>
				</main>
			</div>

			<Footer />
		</div>
	);
}

function Footer() {
	return (
		<footer className="border-[#27272a] border-t bg-black py-16">
			<div className="mx-auto flex w-full max-w-[1400px] flex-col items-center justify-between gap-6 px-6 sm:flex-row lg:px-16">
				<div className="flex items-center gap-3">
					<div className="flex h-6 w-6 items-center justify-center border border-[#27272a] bg-black">
						<svg
							aria-label="Forged logo"
							className="text-white"
							fill="none"
							height="12"
							role="img"
							stroke="currentColor"
							strokeWidth="2"
							viewBox="0 0 24 24"
							width="12"
						>
							<path d="M15 3h6v6" />
							<path d="M10 14L21 3" />
							<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
						</svg>
					</div>
					<span className="font-bold text-[#a1a1aa] text-[11px] uppercase tracking-widest">&copy; Forged Inc {new Date().getFullYear()}</span>
				</div>
				<div className="flex items-center gap-10 text-[#a1a1aa] text-[11px] uppercase tracking-widest">
					<a className="transition-colors hover:text-[#ea580c]" href="https://github.com/itzzritik/forged">
						GitHub
					</a>
					<Link className="transition-colors hover:text-[#ea580c]" href="/docs">
						Docs
					</Link>
					<Link className="transition-colors hover:text-[#ea580c]" href="/security">
						Privacy
					</Link>
				</div>
			</div>
		</footer>
	);
}
