import Link from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Documentation - Forged",
  description: "Installation, setup, and configuration guide for Forged SSH key manager.",
};

function Code({ children }: { children: string }) {
  return (
    <code className="px-1.5 py-0.5 rounded bg-white/5 border border-white/10 text-white text-[13px] font-mono">
      {children}
    </code>
  );
}

function CodeBlock({ title, children }: { title?: string; children: string }) {
  return (
    <div className="rounded border border-white/10 bg-black overflow-hidden my-6 relative group shadow-[0_0_15px_-3px_rgba(255,255,255,0.05)]">
      {title && (
        <div className="flex items-center gap-2 px-4 h-10 border-b border-white/10 bg-white/5">
          <div className="flex gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-white/20 group-hover:bg-red-500/80 transition-colors" />
            <div className="w-2.5 h-2.5 rounded-full bg-white/20 group-hover:bg-yellow-500/80 transition-colors" />
            <div className="w-2.5 h-2.5 rounded-full bg-white/20 group-hover:bg-green-500/80 transition-colors" />
          </div>
          <span className="text-[10px] text-zinc-500 uppercase tracking-widest font-mono select-none">{title}</span>
        </div>
      )}
      <pre className="p-5 text-sm leading-7 overflow-x-auto" style={{ fontFamily: "var(--font-mono)" }}>
        <code className="text-zinc-300">{children}</code>
      </pre>
    </div>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="mb-20 scroll-mt-24 pt-4 border-t border-white/10 first:border-0 first:pt-0">
      <h2 className="text-2xl font-bold tracking-tight mb-8 text-white">{title}</h2>
      {children}
    </section>
  );
}

function TOCLink({ href, children }: { href: string; children: string }) {
  return (
    <a href={href} className="block text-xs uppercase tracking-widest text-zinc-500 hover:text-white transition-colors py-2 font-mono">
      {children}
    </a>
  );
}

export default function DocsPage() {
  return (
    <div className="bg-black min-h-screen text-zinc-400">
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-white/10 bg-black/50 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-6 h-14 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3 hover:opacity-80 transition-opacity">
            <span className="text-sm font-semibold tracking-widest text-white uppercase" style={{ fontFamily: "var(--font-mono)" }}>
              forged
            </span>
          </Link>
          <div className="flex items-center gap-6">
            <Link href="/security" className="text-xs tracking-wider uppercase text-zinc-400 hover:text-white transition-colors" style={{ fontFamily: "var(--font-mono)" }}>
              Security
            </Link>
            <a href="https://github.com/itzzritik/forged" className="text-xs tracking-wider uppercase text-zinc-400 hover:text-white transition-colors" style={{ fontFamily: "var(--font-mono)" }}>
              GitHub
            </a>
          </div>
        </div>
      </nav>

      <div className="max-w-7xl mx-auto px-6 py-32 flex flex-col lg:flex-row gap-16 relative">
        <aside className="hidden lg:block w-56 shrink-0 sticky top-28 self-start pt-2 pr-6 border-r border-white/10 h-[calc(100vh-120px)] overflow-y-auto">
          <div className="text-[10px] text-zinc-600 uppercase tracking-widest font-bold mb-6" style={{ fontFamily: "var(--font-mono)" }}>
            Table of contents
          </div>
          <nav className="space-y-1">
             <TOCLink href="#installation">Installation</TOCLink>
             <TOCLink href="#setup">Setup</TOCLink>
             <TOCLink href="#usage">Usage</TOCLink>
             <TOCLink href="#key-management">Key Management</TOCLink>
             <TOCLink href="#host-matching">Host Matching</TOCLink>
             <TOCLink href="#git-signing">Git Signing</TOCLink>
             <TOCLink href="#sync">Cloud Sync</TOCLink>
             <TOCLink href="#configuration">Configuration</TOCLink>
             <TOCLink href="#commands">Commands Reference</TOCLink>
          </nav>
        </aside>

        <main className="min-w-0 flex-1 lg:max-w-3xl">
          <h1 className="text-5xl font-bold tracking-tighter mb-6 text-white">Documentation</h1>
          <p className="text-zinc-400 text-lg mb-16 leading-relaxed">
            Everything you need to install, configure, and operate Forged.
          </p>

          <Section id="installation" title="Installation">
            <p className="leading-relaxed mb-4">
              Forged is a 15MB single binary compiled purely in Go with zero dependencies.
            </p>
            <CodeBlock title="macOS">{`brew install forged`}</CodeBlock>
            <CodeBlock title="Linux / macOS (BASH)">{`curl -fsSL https://forged.ritik.me/install.sh | sh`}</CodeBlock>
            <CodeBlock title="Compile locally">{`git clone https://github.com/itzzritik/forged
cd forged
just build-cli
./bin/forged setup`}</CodeBlock>
          </Section>

          <Section id="setup" title="Setup Workflow">
            <p className="leading-relaxed mb-4">
              Trigger the initialization wizard which constructs the AES vault, ingests raw plaintext SSH keys from <Code>~/.ssh</Code>, binds the local daemon executable system services, and modifies <Code>~/.ssh/config</Code>.
            </p>
            <CodeBlock>{`forged setup`}</CodeBlock>
            <p className="leading-relaxed mb-4">
              A forced master password ensures cryptographic safety over the encrypted database locally.
            </p>
            <p className="leading-relaxed text-zinc-500">
              The daemon auto-boots on desktop login automatically via launchctl/systemd binding patterns. No external supervision necessary.
            </p>
          </Section>

          <Section id="usage" title="Execution">
            <p className="leading-relaxed mb-4">
              Once bootstrapped, your CLI effectively passes through the Forged agent protocol.
            </p>
            <CodeBlock>{`ssh myserver                     # Resolves automatically
git push origin master           # Implicitly signs the commit`}</CodeBlock>
            <p className="leading-relaxed">
              Compatible across any standard clients observing <Code>SSH_AUTH_SOCK</Code>.
            </p>
          </Section>

          <Section id="key-management" title="Entity Management">
            <CodeBlock>{`forged generate my-key -c "me@host"    # Auto-generates Ed25519
forged add work --file ~/.ssh/id_ed25519  # Ingest
forged list                               # Global index
forged list --json                        # Pipeline indexing
forged export my-key                      # Output stdout PK
forged rename my-key github               # Modify identifier
forged remove old-key                     # Hard delete`}</CodeBlock>
            <p className="leading-relaxed mt-6">
              Migrate bulk payloads seamlessly:
            </p>
            <CodeBlock>{`forged migrate --from ssh          # Pull raw id_rsa/id_ed25519
forged migrate --from 1password    # Integrates via 1P CLI
forged migrate --from agent        # Copies actively bound instances`}</CodeBlock>
          </Section>

          <Section id="host-matching" title="Regex & Host Matching">
             <p className="leading-relaxed mb-4">
              Enforce strict mappings. Banish "Too many authentication attempts" failures entirely.
            </p>
            <CodeBlock>{`forged host github "github.com" "*.github.com"
forged host deploy "*.prod.company.com" "10.0.*"
forged hosts                       # Monitor index definitions
forged unhost deploy "10.0.*"      # Unbind rules`}</CodeBlock>
             <p className="leading-relaxed mt-6">
              Toml definition parameters inside <Code>~/.forged/config.toml</Code>:
            </p>
            <CodeBlock title="config.toml">{`[[hosts]]
name = "GitHub"
match = ["github.com", "*.github.com"]
key = "github"
git_signing = true

[[hosts]]
name = "Production"
match = ["*.prod.company.com", "10.0.*"]
key = "deploy"`}</CodeBlock>
          </Section>

          <Section id="git-signing" title="Signature Verification">
             <CodeBlock title="~/.gitconfig">{`[user]
    signingkey = ssh-ed25519 AAAA...
[gpg]
    format = ssh
[gpg "ssh"]
    program = /usr/local/bin/forged-sign
[commit]
    gpgsign = true`}</CodeBlock>
          </Section>

          <Section id="sync" title="Multi-node Sync">
            <p className="leading-relaxed mb-4">
              Operates over an isolated Blob infrastructure ensuring true zero-knowledge properties across device synchronization matrices.
            </p>
            <CodeBlock>{`forged login                # Init OAuth tokenization
forged sync                 # Propagate vault state
forged sync status          # Monitor sync pipeline
forged logout               # Scrub auth caches`}</CodeBlock>
          </Section>

          <Section id="configuration" title="Core Configurations">
            <ul className="space-y-4 mb-6">
              <li className="flex items-center gap-3"><span className="w-1.5 h-1.5 rounded-full bg-white shrink-0"></span><span className="text-white">macOS:</span> <Code>~/.forged/config.toml</Code></li>
              <li className="flex items-center gap-3"><span className="w-1.5 h-1.5 rounded-full bg-white shrink-0"></span><span className="text-white">Linux:</span> <Code>~/.config/forged/config.toml</Code></li>
            </ul>
            <CodeBlock title="config.toml">{`[agent]
socket = "~/.forged/agent.sock"
log_level = "info"

[sync]
enabled = false`}</CodeBlock>
          </Section>
          
           <Section id="commands" title="Unified Call Stack">
            <CodeBlock>{`# Lifecycle
forged setup                     Bootstrap vault and daemon
forged start / stop              Manage supervisor state
forged status                    Diagnostics block
forged doctor                    System checks and fixes

# Cryptographic
forged generate <name>           Yield new RSA/Ed keys
forged add <name> --file <path>  Consume target
forged list                      Dump database
forged remove <name>             Nuke entry
forged export <name>             Raw out PK

# Routing
forged host <key> <patterns>     Define routing target
forged hosts                     List logic blocks
forged unhost <key> <pattern>    Destruct constraint

# Network
forged login                     Browser OAuth
forged sync                      Execute Blob pipeline 
forged sync status               Monitor
forged logout                    Clear tokens

# Maintenance
forged lock / unlock             Manage buffer suspension
forged change-password           Reroll Argon2 derivation
forged migrate --from <source>   Intake routine
forged benchmark                 Speedtest Argon thresholds
forged logs                      Daemon trace
forged config                    Modify application state`}</CodeBlock>
          </Section>
        </main>
      </div>
    </div>
  );
}
