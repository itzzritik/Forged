import Link from "next/link";
import type { Metadata } from "next";
import { ScrollReveal } from "@/components/client";

export const metadata: Metadata = {
  title: "Documentation - Forged",
  description: "Installation, setup, and configuration guide for Forged SSH key manager.",
};

function Code({ children }: { children: string }) {
  return (
    <code className="px-1.5 py-0.5 bg-black border border-[#27272a] text-[#ea580c] text-[13px] font-mono leading-none inline-flex items-center align-middle -translate-y-px shadow-[4px_4px_0px_rgba(39,39,42,1)]">
      {children}
    </code>
  );
}

function CodeBlock({ title, children }: { title?: string; children: string }) {
  return (
    <div className="border border-[#27272a] bg-black overflow-hidden my-8 relative flex flex-col group relative">
      {/* Background internal glow */}
      <div className="absolute inset-0 bg-gradient-to-br from-[#ea580c]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />

      {title && (
        <div className="flex items-center justify-between px-5 h-12 border-b border-[#27272a] bg-[#09090b]">
          <div className="flex items-center gap-3">
             <span className="h-1.5 w-1.5 bg-[#ea580c]" />
             <span className="text-[#a1a1aa] font-mono text-[10px] tracking-[0.2em] uppercase">SYSTEM // {title}</span>
          </div>
          <span className="text-[10px] text-[#3f3f46] uppercase tracking-widest font-mono select-none">
            READY
          </span>
        </div>
      )}
      <div className="relative flex">
        <div className="hidden sm:flex flex-col items-end w-12 shrink-0 py-5 pr-4 border-r border-[#27272a] bg-[#09090b] text-[11px] text-[#3f3f46] font-mono select-none pointer-events-none">
          {children.split("\n").map((_, i) => (
             <span key={i} className="leading-7">{i + 1}</span>
          ))}
        </div>
        <pre className="p-5 sm:pl-6 text-[13px] leading-7 overflow-x-auto font-mono text-white flex-1">
          <code>{children}</code>
        </pre>
      </div>
    </div>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="mb-32 scroll-mt-32 border-t border-[#27272a] pt-12 first:border-0 first:pt-0">
      <div className="flex items-center gap-2.5 mb-8">
        <span className="w-1.5 h-3 bg-[#ea580c]" />
        <h2 className="text-3xl font-bold tracking-tight text-white m-0">
          {title}
        </h2>
      </div>
      <div className="text-[#a1a1aa] leading-relaxed text-lg space-y-6">
        {children}
      </div>
    </section>
  );
}

function TOCLink({ href, children }: { href: string; children: string }) {
  return (
    <a href={href} className="group flex items-center justify-between text-xs tracking-widest text-[#a1a1aa] hover:text-white transition-colors py-2 font-mono">
      <span className="flex items-center gap-3 uppercase">
        <span className="text-[#3f3f46] group-hover:text-[#ea580c] transition-colors">{">"}</span>
        {children}
      </span>
      <span className="text-[#ea580c] opacity-0 group-hover:opacity-100 transition-opacity">●</span>
    </a>
  );
}

export default function DocsPage() {
  return (
    <div className="bg-black min-h-screen text-[#a1a1aa] relative overflow-clip">
      {/* Brutalist Repeating Background */}
      <div className="pointer-events-none fixed inset-0 opacity-[0.03]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />
      <div className="fixed inset-y-0 left-0 w-8 bg-gradient-to-r from-black to-transparent pointer-events-none z-10" />

      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-[#27272a] bg-black/80 backdrop-blur-xl">
        <div className="w-full px-6 lg:px-16 h-14 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3 group">
            <div className="w-7 h-7 bg-black border border-[#27272a] flex items-center justify-center group-hover:border-[#ea580c] transition-colors">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-white group-hover:text-[#ea580c] transition-colors">
                <path d="M15 3h6v6" />
                <path d="M10 14L21 3" />
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
              </svg>
            </div>
            <span className="text-[13px] font-bold tracking-[0.2em] text-white uppercase font-mono group-hover:text-[#ea580c] transition-colors">
              Forged
            </span>
          </Link>
          <div className="flex items-center gap-8">
            <Link href="/security" className="text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
              Security
            </Link>
            <a href="https://github.com/itzzritik/forged" className="text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
              GitHub
            </a>
          </div>
        </div>
      </nav>

      <div className="relative z-10 w-full max-w-[1400px] mx-auto px-6 lg:px-16 pt-32 pb-32 flex flex-col lg:flex-row gap-20">
        <aside className="hidden lg:block w-72 shrink-0 sticky top-32 self-start pt-2 pr-12 h-[calc(100vh-140px)] overflow-y-auto">
          <div className="flex flex-col gap-12">
            <div>
              <div className="text-[10px] text-[#ea580c] uppercase tracking-[0.2em] font-bold mb-6 font-mono pb-2 border-b border-[#27272a]">
                [ 01 ] // Getting Started
              </div>
              <nav className="flex flex-col space-y-1">
                <TOCLink href="#installation">Installation</TOCLink>
                <TOCLink href="#setup">Setup Workflow</TOCLink>
                <TOCLink href="#usage">Execution</TOCLink>
              </nav>
            </div>

            <div>
              <div className="text-[10px] text-[#ea580c] uppercase tracking-[0.2em] font-bold mb-6 font-mono pb-2 border-b border-[#27272a]">
                [ 02 ] // Core Concepts
              </div>
              <nav className="flex flex-col space-y-1">
                <TOCLink href="#key-management">Entity Management</TOCLink>
                <TOCLink href="#host-matching">Host Matching</TOCLink>
                <TOCLink href="#git-signing">Git Signing</TOCLink>
              </nav>
            </div>

            <div>
              <div className="text-[10px] text-[#ea580c] uppercase tracking-[0.2em] font-bold mb-6 font-mono pb-2 border-b border-[#27272a]">
                [ 03 ] // Advanced Setup
              </div>
              <nav className="flex flex-col space-y-1">
                <TOCLink href="#sync">Cloud Sync</TOCLink>
                <TOCLink href="#configuration">Configuration</TOCLink>
                <TOCLink href="#commands">Commands Ref</TOCLink>
              </nav>
            </div>
          </div>
        </aside>

        <main className="min-w-0 flex-[1.5] max-w-4xl pt-2 pb-8">
          <ScrollReveal className="mb-16">
            <div className="flex items-center gap-2.5 mb-6">
              <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Operations Reference</span>
            </div>
            <h1 className="text-[clamp(3.5rem,6vw,7rem)] font-bold tracking-tighter mb-8 text-white leading-[0.9] text-pretty">
              Documentation.
            </h1>
            <p className="text-[#a1a1aa] text-lg lg:text-xl leading-relaxed max-w-2xl">
              Strictly rigorous infrastructure guidelines spanning installation, network configuration, and operational commands for the Forged daemon.
            </p>
          </ScrollReveal>

          <Section id="installation" title="Installation">
            <p>
              Forged is distributed as a highly optimized, 15MB single binary compiled purely in Go with zero external CGO dependencies.
            </p>
            <CodeBlock title="macOS">brew install forged</CodeBlock>
            <CodeBlock title="Linux / macOS (BASH)">{"curl -fsSL https://forged.ritik.me/install.sh | sh"}</CodeBlock>
            <CodeBlock title="Compile locally">{"git clone https://github.com/itzzritik/forged\ncd forged\njust build-cli\n./bin/forged setup"}</CodeBlock>
          </Section>

          <Section id="setup" title="Setup Workflow">
            <p>
              Execute the initialization wizard to construct the AES vault, ingest your raw plaintext SSH keys from <Code>~/.ssh</Code>, bind the local daemon executable system services, and modify <Code>~/.ssh/config</Code>.
            </p>
            <CodeBlock title="Terminal">forged setup</CodeBlock>
            
            {/* Warning Diagnostic Alert */}
            <div className="p-6 bg-black border border-[#27272a] shadow-[4px_4px_0px_#ea580c] relative overflow-hidden group my-10">
              <div className="flex items-center gap-3 mb-4 border-b border-[#27272a] pb-4">
                 <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#ea580c" strokeWidth="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg>
                 <span className="text-[#ea580c] text-[11px] font-mono tracking-[0.2em] font-bold uppercase">System Diagnostic Warning</span>
              </div>
              <p className="m-0 text-sm md:text-base text-white/90 leading-relaxed">
                 A mandatory master password ensures cryptographic safety over the database locally using <span className="text-[#ea580c]">Argon2id</span> derivation. Protect this passphrase strictly.
              </p>
            </div>
            
            <p className="text-white/70">
              The daemon auto-boots on desktop login automatically via launchctl/systemd binding patterns. No external supervision necessary.
            </p>
          </Section>

          <Section id="usage" title="Execution">
            <p>
              Once bootstrapped, your CLI effectively passes through the Forged agent protocol. Compatible across any standard clients observing <Code>SSH_AUTH_SOCK</Code>.
            </p>
            <CodeBlock title="Workflow">{"$ ssh myserver                     # Resolves automatically\n$ git push origin master           # Implicitly signs the commit"}</CodeBlock>
          </Section>

          <Section id="key-management" title="Entity Management">
            <p>Manage the lifecycle of keys directly inside the vault without ever touching the filesystem in plaintext.</p>
            <CodeBlock title="Management">{"$ forged generate my-key -c \"me@host\"    # Auto-generates Ed25519\n$ forged add work --file ~/.ssh/id_ed25519  # Ingest existing payload\n$ forged list                               # Global index status\n$ forged list --json                        # CI Pipeline indexing\n$ forged export my-key                      # Output stdout PK\n$ forged rename my-key github               # Modify identifier\n$ forged remove old-key                     # Hard delete entity"}</CodeBlock>
            <p className="mt-8 border-l border-[#ea580c]/50 pl-4">
              Migrate bulk payloads seamlessly from existing agents using ingestion protocols:
            </p>
            <CodeBlock title="Migration Protocol">{"$ forged migrate --from ssh          # Pull raw id_rsa/id_ed25519 payloads directly\n$ forged migrate --from 1password    # Directly integrates via 1P CLI interface\n$ forged migrate --from agent        # Copies actively bound instances loaded in memory"}</CodeBlock>
          </Section>

          <Section id="host-matching" title="Regex & Host Matching">
            <p>
              Enforce strict mappings computationally. Banish &quot;Too many authentication attempts&quot; failures entirely by binding specific keys exclusively to specific domains.
            </p>
            <CodeBlock title="Routing Configuration">{"$ forged host github \"github.com\" \"*.github.com\"\n$ forged host deploy \"*.prod.company.com\" \"10.0.*\"\n$ forged hosts                       # Monitor index definitions active in daemon\n$ forged unhost deploy \"10.0.*\"      # Unbind rules gracefully"}</CodeBlock>
            <p className="mt-8 text-white/50 text-sm uppercase tracking-widest font-mono">
              [ Manual overrides via local architecture ]
            </p>
            <p className="mt-4">
               Alternatively, you can manually define patterns inside your local <Code>~/.forged/config.toml</Code>:
            </p>
            <CodeBlock title="config.toml">{"[[hosts]]\nname = \"GitHub\"\nmatch = [\"github.com\", \"*.github.com\"]\nkey = \"github\"\ngit_signing = true\n\n[[hosts]]\nname = \"Production\"\nmatch = [\"*.prod.company.com\", \"10.0.*\"]\nkey = \"deploy\""}</CodeBlock>
          </Section>

          <Section id="git-signing" title="Signature Verification">
             <p>Enable rigorous provenance tracing by utilizing SSH signatures instead of traditional GPG protocols.</p>
            <CodeBlock title="~/.gitconfig">{"[user]\n    signingkey = ssh-ed25519 AAAA...\n[gpg]\n    format = ssh\n[gpg \"ssh\"]\n    program = /usr/local/bin/forged-sign\n[commit]\n    gpgsign = true"}</CodeBlock>
          </Section>

          <Section id="sync" title="Multi-node Sync">
            <p>
              Operates over an isolated Blob infrastructure ensuring true zero-knowledge properties across device synchronization matrices.
            </p>
            <CodeBlock title="Sync Pipeline">{"$ forged login                # Init OAuth tokenization payload\n$ forged sync                 # Propagate full vault state\n$ forged sync status          # Monitor sync pipeline operations\n$ forged logout               # Scrub auth caches thoroughly"}</CodeBlock>
          </Section>

          <Section id="configuration" title="Core Configurations">
            <ul className="space-y-4 mb-8 font-mono text-sm border border-[#27272a] p-6 bg-black shadow-[4px_4px_0px_rgba(39,39,42,1)]">
              <li className="flex items-center gap-4"><span className="w-2 h-2 rounded-full bg-[#ea580c] shrink-0" /><span className="text-[#a1a1aa] min-w-[70px]">macOS:</span> <span className="text-white">~/.forged/config.toml</span></li>
              <li className="flex items-center gap-4"><span className="w-2 h-2 rounded-full bg-[#ea580c] shrink-0" /><span className="text-[#a1a1aa] min-w-[70px]">Linux:</span> <span className="text-white">~/.config/forged/config.toml</span></li>
            </ul>
            <CodeBlock title="config.toml">{"[agent]\nsocket = \"~/.forged/agent.sock\"\nlog_level = \"info\"\n\n[sync]\nenabled = false"}</CodeBlock>
          </Section>

          <Section id="commands" title="Unified Call Stack">
            <CodeBlock title="CLI Reference">{"# Lifecycle\nforged setup                     Bootstrap vault and daemon\nforged start / stop              Manage supervisor state\nforged status                    Diagnostics block\nforged doctor                    System checks and fixes\n\n# Cryptographic\nforged generate <name>           Yield new RSA/Ed keys\nforged add <name> --file <path>  Consume target\nforged list                      Dump database\nforged remove <name>             Nuke entry\nforged export <name>             Raw out PK\n\n# Routing\nforged host <key> <patterns>     Define routing target\nforged hosts                     List logic blocks\nforged unhost <key> <pattern>    Destruct constraint\n\n# Network\nforged login                     Browser OAuth\nforged sync                      Execute Blob pipeline \nforged sync status               Monitor\nforged logout                    Clear tokens\n\n# Maintenance\nforged lock / unlock             Manage buffer suspension\nforged change-password           Reroll Argon2 derivation\nforged migrate --from <source>   Intake routine\nforged benchmark                 Speedtest Argon thresholds\nforged logs                      Daemon trace\nforged config                    Modify application state"}</CodeBlock>
          </Section>
        </main>
      </div>

      <Footer />
    </div>
  );
}

function Footer() {
  return (
    <footer className="py-16 bg-black border-t border-[#27272a]">
      <div className="w-full max-w-[1400px] mx-auto px-6 lg:px-16 flex flex-col sm:flex-row items-center justify-between gap-6">
        <div className="flex items-center gap-3">
          <div className="w-6 h-6 bg-black border border-[#27272a] flex items-center justify-center">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-white">
              <path d="M15 3h6v6" />
              <path d="M10 14L21 3" />
              <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
            </svg>
          </div>
          <span className="text-[11px] uppercase font-bold tracking-widest text-[#a1a1aa]">
            &copy; Forged Inc {new Date().getFullYear()}
          </span>
        </div>
        <div className="flex items-center gap-10 text-[11px] uppercase tracking-widest text-[#a1a1aa]">
          <a href="https://github.com/itzzritik/forged" className="hover:text-[#ea580c] transition-colors">
            GitHub
          </a>
          <Link href="/docs" className="hover:text-[#ea580c] transition-colors">
            Docs
          </Link>
          <Link href="/security" className="hover:text-[#ea580c] transition-colors">
            Privacy
          </Link>
        </div>
      </div>
    </footer>
  );
}

