import Link from "next/link";
import type { Metadata } from "next";
import { ScrollReveal } from "@/components/client";

export const metadata: Metadata = {
  title: "Documentation - Forged",
  description: "Installation, setup, and configuration guide for Forged SSH key manager.",
};

function Code({ children }: { children: string }) {
  return (
    <code className="px-1.5 py-0.5 bg-[#09090b] border border-[#27272a] text-[#ea580c] text-[13px] font-mono leading-none inline-flex items-center align-middle -translate-y-px">
      {children}
    </code>
  );
}

function CodeBlock({ title, children }: { title?: string; children: string }) {
  return (
    <div className="border border-[#27272a] bg-black overflow-hidden my-8 relative group shadow-2xl">
      {title && (
        <div className="flex items-center justify-between px-4 h-11 border-b border-[#27272a] bg-[#09090b]">
          <div className="flex items-center gap-6">
            <div className="flex gap-2">
              <div className="w-3 h-3 rounded-full bg-[#27272a] group-hover:bg-[#FF5F56] transition-colors duration-300" />
              <div className="w-3 h-3 rounded-full bg-[#27272a] group-hover:bg-[#FFBD2E] transition-colors duration-300" />
              <div className="w-3 h-3 rounded-full bg-[#27272a] group-hover:bg-[#27C93F] transition-colors duration-300" />
            </div>
            <span className="text-white border-b border-[#ea580c] pb-2.5 translate-y-[6px] text-xs font-mono">
              {title}
            </span>
          </div>
          <span className="text-[10px] text-[#27272a] uppercase tracking-widest font-mono select-none flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-[#ea580c] animate-pulse" />
            Active
          </span>
        </div>
      )}
      <div className="relative">
        <div className="absolute top-0 left-0 bottom-0 w-12 border-r border-[#27272a]/50 bg-black/50 hidden sm:flex flex-col items-center py-5 text-[10px] text-[#27272a] font-mono select-none pointer-events-none">
          {children.split("\\n").map((_, i) => (
            <span key={i} className="leading-7">{i + 1}</span>
          ))}
        </div>
        <pre className="p-5 sm:pl-16 text-[13px] leading-7 overflow-x-auto relative font-mono">
          <code className="text-white">{children}</code>
        </pre>
      </div>
    </div>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="mb-24 scroll-mt-24 pt-4 border-t border-[#27272a] first:border-0 first:pt-0">
      <h2 className="text-3xl font-bold tracking-tight mb-8 text-white relative inline-block">
        {title}
        <div className="absolute -bottom-2 left-0 w-1/3 h-px bg-[#ea580c]" />
      </h2>
      <div className="text-[#a1a1aa] leading-relaxed text-lg space-y-6">
        {children}
      </div>
    </section>
  );
}

function TOCLink({ href, children }: { href: string; children: string }) {
  return (
    <a href={href} className="group flex items-center gap-3 text-xs uppercase tracking-widest text-[#a1a1aa] hover:text-white transition-colors py-2.5 font-mono">
      <span className="w-1 h-1 bg-[#27272a] group-hover:bg-[#ea580c] transition-colors" />
      {children}
    </a>
  );
}

export default function DocsPage() {
  return (
    <div className="bg-black min-h-screen text-[#a1a1aa]">
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-[#27272a] bg-black/80 backdrop-blur-xl">
        <div className="max-w-7xl mx-auto px-6 h-14 flex items-center justify-between">
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

      <div className="max-w-7xl mx-auto px-6 pt-32 pb-20 flex flex-col lg:flex-row gap-20 relative">
        <aside className="hidden lg:block w-64 shrink-0 sticky top-32 self-start pt-2 pr-6 border-r border-[#27272a] h-[calc(100vh-140px)] overflow-y-auto">
          <div className="flex flex-col gap-8">
            <div>
              <div className="text-[10px] text-[#27272a] uppercase tracking-widest font-bold mb-6 flex items-center gap-3 font-mono">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="17 8 12 3 7 8" /><line x1="12" y1="3" x2="12" y2="15" /></svg>
                Getting Started
              </div>
              <nav className="space-y-1 pl-2">
                <TOCLink href="#installation">Installation</TOCLink>
                <TOCLink href="#setup">Setup Workflow</TOCLink>
                <TOCLink href="#usage">Execution</TOCLink>
              </nav>
            </div>

            <div>
              <div className="text-[10px] text-[#27272a] uppercase tracking-widest font-bold mb-6 flex items-center gap-3 font-mono">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>
                Core Concepts
              </div>
              <nav className="space-y-1 pl-2">
                <TOCLink href="#key-management">Entity Management</TOCLink>
                <TOCLink href="#host-matching">Host Matching</TOCLink>
                <TOCLink href="#git-signing">Git Signing</TOCLink>
              </nav>
            </div>

            <div>
              <div className="text-[10px] text-[#27272a] uppercase tracking-widest font-bold mb-6 flex items-center gap-3 font-mono">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12" /></svg>
                Advanced
              </div>
              <nav className="space-y-1 pl-2">
                <TOCLink href="#sync">Cloud Sync</TOCLink>
                <TOCLink href="#configuration">Configuration</TOCLink>
                <TOCLink href="#commands">Commands Ref</TOCLink>
              </nav>
            </div>
          </div>
        </aside>

        <main className="min-w-0 flex-[1.5] max-w-4xl pt-2">
          <ScrollReveal className="mb-16">
            <h1 className="text-5xl md:text-7xl font-bold tracking-tighter mb-8 text-white relative inline-block leading-none">
              Documentation.
              <div className="absolute -inset-4 bg-[#ea580c]/8 blur-3xl -z-10" />
            </h1>
            <p className="text-[#a1a1aa] text-xl leading-relaxed max-w-2xl">
              Everything you need to install, configure, and operate the Forged SSH daemon effectively within your infrastructure.
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
            <div className="p-6 border-l-2 border-[#ea580c] bg-[#ea580c]/5 my-8">
              <p className="m-0 text-white font-semibold">Security Note</p>
              <p className="m-0 text-sm mt-2 text-[#a1a1aa]">A mandatory master password ensures cryptographic safety over the database locally using Argon2id deriving.</p>
            </div>
            <p className="text-[#27272a] text-sm">
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
            <p>Manage the lifecycle of keys directly inside the vault without ever touching the filesystem.</p>
            <CodeBlock title="Management">{"$ forged generate my-key -c \"me@host\"    # Auto-generates Ed25519\n$ forged add work --file ~/.ssh/id_ed25519  # Ingest\n$ forged list                               # Global index\n$ forged list --json                        # Pipeline indexing\n$ forged export my-key                      # Output stdout PK\n$ forged rename my-key github               # Modify identifier\n$ forged remove old-key                     # Hard delete"}</CodeBlock>
            <p className="mt-8">
              Migrate bulk payloads seamlessly from existing agents:
            </p>
            <CodeBlock title="Migration">{"$ forged migrate --from ssh          # Pull raw id_rsa/id_ed25519\n$ forged migrate --from 1password    # Integrates via 1P CLI\n$ forged migrate --from agent        # Copies actively bound instances"}</CodeBlock>
          </Section>

          <Section id="host-matching" title="Regex & Host Matching">
            <p>
              Enforce strict mappings. Banish &quot;Too many authentication attempts&quot; failures entirely by binding specific keys exclusively to specific domains.
            </p>
            <CodeBlock title="Routing">{"$ forged host github \"github.com\" \"*.github.com\"\n$ forged host deploy \"*.prod.company.com\" \"10.0.*\"\n$ forged hosts                       # Monitor index definitions\n$ forged unhost deploy \"10.0.*\"      # Unbind rules"}</CodeBlock>
            <p className="mt-8">
              Alternatively, you can manually define patterns inside your local <Code>~/.forged/config.toml</Code>:
            </p>
            <CodeBlock title="config.toml">{"[[hosts]]\nname = \"GitHub\"\nmatch = [\"github.com\", \"*.github.com\"]\nkey = \"github\"\ngit_signing = true\n\n[[hosts]]\nname = \"Production\"\nmatch = [\"*.prod.company.com\", \"10.0.*\"]\nkey = \"deploy\""}</CodeBlock>
          </Section>

          <Section id="git-signing" title="Signature Verification">
            <CodeBlock title="~/.gitconfig">{"[user]\n    signingkey = ssh-ed25519 AAAA...\n[gpg]\n    format = ssh\n[gpg \"ssh\"]\n    program = /usr/local/bin/forged-sign\n[commit]\n    gpgsign = true"}</CodeBlock>
          </Section>

          <Section id="sync" title="Multi-node Sync">
            <p>
              Operates over an isolated Blob infrastructure ensuring true zero-knowledge properties across device synchronization matrices.
            </p>
            <CodeBlock title="Sync Pipeline">{"$ forged login                # Init OAuth tokenization\n$ forged sync                 # Propagate vault state\n$ forged sync status          # Monitor sync pipeline\n$ forged logout               # Scrub auth caches"}</CodeBlock>
          </Section>

          <Section id="configuration" title="Core Configurations">
            <ul className="space-y-4 mb-6">
              <li className="flex items-center gap-3"><span className="w-1.5 h-1.5 bg-white shrink-0" /><span className="text-white">macOS:</span> <Code>~/.forged/config.toml</Code></li>
              <li className="flex items-center gap-3"><span className="w-1.5 h-1.5 bg-white shrink-0" /><span className="text-white">Linux:</span> <Code>~/.config/forged/config.toml</Code></li>
            </ul>
            <CodeBlock title="config.toml">{"[agent]\nsocket = \"~/.forged/agent.sock\"\nlog_level = \"info\"\n\n[sync]\nenabled = false"}</CodeBlock>
          </Section>

          <Section id="commands" title="Unified Call Stack">
            <CodeBlock title="CLI Reference">{"# Lifecycle\nforged setup                     Bootstrap vault and daemon\nforged start / stop              Manage supervisor state\nforged status                    Diagnostics block\nforged doctor                    System checks and fixes\n\n# Cryptographic\nforged generate <name>           Yield new RSA/Ed keys\nforged add <name> --file <path>  Consume target\nforged list                      Dump database\nforged remove <name>             Nuke entry\nforged export <name>             Raw out PK\n\n# Routing\nforged host <key> <patterns>     Define routing target\nforged hosts                     List logic blocks\nforged unhost <key> <pattern>    Destruct constraint\n\n# Network\nforged login                     Browser OAuth\nforged sync                      Execute Blob pipeline \nforged sync status               Monitor\nforged logout                    Clear tokens\n\n# Maintenance\nforged lock / unlock             Manage buffer suspension\nforged change-password           Reroll Argon2 derivation\nforged migrate --from <source>   Intake routine\nforged benchmark                 Speedtest Argon thresholds\nforged logs                      Daemon trace\nforged config                    Modify application state"}</CodeBlock>
          </Section>
        </main>
      </div>
    </div>
  );
}
