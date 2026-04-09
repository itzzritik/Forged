import Link from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Documentation - Forged",
  description: "Installation, setup, and configuration guide for Forged SSH key manager.",
};

function Code({ children }: { children: string }) {
  return (
    <code className="px-1.5 py-0.5 rounded bg-surface border border-border text-zinc-300 text-[13px]">
      {children}
    </code>
  );
}

function CodeBlock({ title, children }: { title?: string; children: string }) {
  return (
    <div className="rounded-xl bg-surface border border-border overflow-hidden my-4">
      {title && (
        <div className="flex items-center gap-2 px-4 h-9 border-b border-border">
          <div className="flex gap-1.5">
            <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
            <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
            <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
          </div>
          <span className="text-xs text-muted ml-2">{title}</span>
        </div>
      )}
      <pre className="p-4 text-sm leading-7 overflow-x-auto" style={{ fontFamily: "var(--font-mono)" }}>
        <code className="text-zinc-300">{children}</code>
      </pre>
    </div>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="mb-16 scroll-mt-20">
      <h2 className="text-xl font-medium tracking-tight mb-4">{title}</h2>
      {children}
    </section>
  );
}

function TOCLink({ href, children }: { href: string; children: string }) {
  return (
    <a href={href} className="block text-sm text-muted hover:text-foreground transition-colors py-1">
      {children}
    </a>
  );
}

export default function DocsPage() {
  return (
    <>
      <nav className="w-full border-b border-border">
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3">
            <span className="text-sm font-medium tracking-tight" style={{ fontFamily: "var(--font-mono)" }}>
              forged
            </span>
          </Link>
          <div className="flex items-center gap-6">
            <Link href="/security" className="text-sm text-muted hover:text-foreground transition-colors">
              Security
            </Link>
            <a href="https://github.com/itzzritik/forged" className="text-sm text-muted hover:text-foreground transition-colors">
              GitHub
            </a>
          </div>
        </div>
      </nav>

      <div className="max-w-5xl mx-auto px-6 py-16 flex gap-16">
        <aside className="hidden lg:block w-48 shrink-0 sticky top-16 self-start">
          <div className="text-xs text-muted uppercase tracking-widest mb-4" style={{ fontFamily: "var(--font-mono)" }}>
            On this page
          </div>
          <TOCLink href="#installation">Installation</TOCLink>
          <TOCLink href="#setup">Setup</TOCLink>
          <TOCLink href="#usage">Usage</TOCLink>
          <TOCLink href="#key-management">Key Management</TOCLink>
          <TOCLink href="#host-matching">Host Matching</TOCLink>
          <TOCLink href="#git-signing">Git Signing</TOCLink>
          <TOCLink href="#sync">Cloud Sync</TOCLink>
          <TOCLink href="#configuration">Configuration</TOCLink>
          <TOCLink href="#commands">All Commands</TOCLink>
        </aside>

        <main className="min-w-0 flex-1">
          <h1 className="text-3xl font-medium tracking-tight mb-4">Documentation</h1>
          <p className="text-muted mb-12 leading-relaxed">
            Everything you need to install, configure, and use Forged.
          </p>

          <Section id="installation" title="Installation">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Forged is a single binary with no dependencies.
            </p>
            <CodeBlock title="macOS">{`brew install forged`}</CodeBlock>
            <CodeBlock title="Linux / macOS (script)">{`curl -fsSL https://forged.ritik.me/install.sh | sh`}</CodeBlock>
            <CodeBlock title="From source">{`git clone https://github.com/itzzritik/forged
cd forged
just build-cli
./bin/forged setup`}</CodeBlock>
          </Section>

          <Section id="setup" title="Setup">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Run the setup wizard. It creates an encrypted vault, imports your existing SSH keys,
              installs the daemon as a system service, and configures <Code>~/.ssh/config</Code>.
            </p>
            <CodeBlock>{`forged setup`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mb-4">
              You will be asked to create a master password. This encrypts your vault locally.
              It is never sent to any server.
            </p>
            <p className="text-sm text-muted leading-relaxed">
              After setup, the daemon starts automatically and runs in the background.
              It auto-starts on login via launchd (macOS) or systemd (Linux).
            </p>
          </Section>

          <Section id="usage" title="Usage">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Once the daemon is running, SSH and Git work automatically.
              Forged serves keys over the standard SSH agent protocol.
            </p>
            <CodeBlock>{`ssh myserver                     # right key, automatically
git push origin main             # commits signed, automatically`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed">
              Any SSH client that supports <Code>SSH_AUTH_SOCK</Code> works with Forged.
              You can verify with <Code>ssh-add -l</Code>.
            </p>
          </Section>

          <Section id="key-management" title="Key Management">
            <CodeBlock>{`forged generate my-key -c "me@host"    # new Ed25519 key
forged add work --file ~/.ssh/id_ed25519  # import existing
forged list                               # show all keys
forged list --json                        # machine-readable
forged export my-key                      # public key to stdout
forged rename my-key github               # rename
forged remove old-key                     # delete`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              To import keys from 1Password or your existing SSH agent:
            </p>
            <CodeBlock>{`forged migrate --from ssh          # import from ~/.ssh/
forged migrate --from 1password    # import from 1Password CLI
forged migrate --from agent        # list keys in current agent`}</CodeBlock>
          </Section>

          <Section id="host-matching" title="Host Matching">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Map keys to hosts so the right key is always offered first. Supports exact hostnames,
              wildcards, IP ranges, and regex.
            </p>
            <CodeBlock>{`forged host github "github.com" "*.github.com"
forged host deploy "*.prod.company.com" "10.0.*"
forged hosts                       # list all mappings
forged unhost deploy "10.0.*"      # remove a mapping`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              You can also configure host rules in <Code>~/.forged/config.toml</Code>:
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

          <Section id="git-signing" title="Git Signing">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Forged can sign your Git commits with SSH keys. Run <Code>forged setup</Code> and
              say yes to Git signing, or configure manually:
            </p>
            <CodeBlock title="~/.gitconfig">{`[user]
    signingkey = ssh-ed25519 AAAA...
[gpg]
    format = ssh
[gpg "ssh"]
    program = /usr/local/bin/forged-sign
[commit]
    gpgsign = true`}</CodeBlock>
          </Section>

          <Section id="sync" title="Cloud Sync">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Sync your encrypted vault across machines. The server stores only opaque blobs
              it cannot decrypt.
            </p>
            <CodeBlock>{`forged login                # opens browser for OAuth
forged sync                 # push/pull vault
forged sync status          # check sync state
forged logout               # clear credentials`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              On a new machine, install Forged, run <Code>forged login</Code> and <Code>forged sync</Code>,
              then enter your master password to decrypt the vault. All keys are available.
            </p>
          </Section>

          <Section id="configuration" title="Configuration">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Configuration file location:
            </p>
            <ul className="text-sm text-muted space-y-1 mb-4">
              <li>macOS: <Code>~/.forged/config.toml</Code></li>
              <li>Linux: <Code>~/.config/forged/config.toml</Code></li>
            </ul>
            <CodeBlock title="config.toml">{`[agent]
socket = "~/.forged/agent.sock"
log_level = "info"

[sync]
enabled = false`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              Use <Code>forged config</Code> to open the config file in your editor,
              or <Code>forged config get</Code> / <Code>forged config set</Code> to manage individual values.
            </p>
          </Section>

          <Section id="commands" title="All Commands">
            <CodeBlock>{`forged setup                     first-time wizard
forged start / stop              manage daemon service
forged status                    daemon info + key count
forged doctor                    diagnose common issues

forged generate <name>           new Ed25519 key pair
forged add <name> --file <path>  import existing key
forged list                      all keys in vault
forged remove <name>             delete a key
forged export <name>             public key to stdout
forged rename <old> <new>        rename a key

forged host <key> <patterns>     map key to hosts
forged hosts                     list all mappings
forged unhost <key> <pattern>    remove a mapping

forged login                     authenticate with cloud
forged sync                      push/pull encrypted vault
forged sync status               check sync state
forged logout                    clear credentials

forged lock / unlock             clear or restore keys
forged change-password           change master password
forged migrate --from <source>   import from ssh/1password/agent
forged benchmark                 test Argon2id speed
forged logs                      tail daemon logs
forged config                    manage configuration`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              All commands support <Code>--json</Code> for machine-readable output.
            </p>
          </Section>
        </main>
      </div>
    </>
  );
}
        </div>
      </nav>

      <div className="max-w-5xl mx-auto px-6 py-16 flex gap-16">
        <aside className="hidden lg:block w-48 shrink-0 sticky top-16 self-start">
          <div className="text-xs text-muted uppercase tracking-widest mb-4" style={{ fontFamily: "var(--font-mono)" }}>
            On this page
          </div>
          <TOCLink href="#installation">Installation</TOCLink>
          <TOCLink href="#setup">Setup</TOCLink>
          <TOCLink href="#usage">Usage</TOCLink>
          <TOCLink href="#key-management">Key Management</TOCLink>
          <TOCLink href="#host-matching">Host Matching</TOCLink>
          <TOCLink href="#git-signing">Git Signing</TOCLink>
          <TOCLink href="#sync">Cloud Sync</TOCLink>
          <TOCLink href="#configuration">Configuration</TOCLink>
          <TOCLink href="#commands">All Commands</TOCLink>
        </aside>

        <main className="min-w-0 flex-1">
          <h1 className="text-3xl font-medium tracking-tight mb-4">Documentation</h1>
          <p className="text-muted mb-12 leading-relaxed">
            Everything you need to install, configure, and use Forged.
          </p>

          <Section id="installation" title="Installation">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Forged is a single binary with no dependencies.
            </p>
            <CodeBlock title="macOS">{`brew install forged`}</CodeBlock>
            <CodeBlock title="Linux / macOS (script)">{`curl -fsSL https://forged.ritik.me/install.sh | sh`}</CodeBlock>
            <CodeBlock title="From source">{`git clone https://github.com/itzzritik/forged
cd forged
just build-cli
./bin/forged setup`}</CodeBlock>
          </Section>

          <Section id="setup" title="Setup">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Run the setup wizard. It creates an encrypted vault, imports your existing SSH keys,
              installs the daemon as a system service, and configures <Code>~/.ssh/config</Code>.
            </p>
            <CodeBlock>{`forged setup`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mb-4">
              You will be asked to create a master password. This encrypts your vault locally.
              It is never sent to any server.
            </p>
            <p className="text-sm text-muted leading-relaxed">
              After setup, the daemon starts automatically and runs in the background.
              It auto-starts on login via launchd (macOS) or systemd (Linux).
            </p>
          </Section>

          <Section id="usage" title="Usage">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Once the daemon is running, SSH and Git work automatically.
              Forged serves keys over the standard SSH agent protocol.
            </p>
            <CodeBlock>{`ssh myserver                     # right key, automatically
git push origin main             # commits signed, automatically`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed">
              Any SSH client that supports <Code>SSH_AUTH_SOCK</Code> works with Forged.
              You can verify with <Code>ssh-add -l</Code>.
            </p>
          </Section>

          <Section id="key-management" title="Key Management">
            <CodeBlock>{`forged generate my-key -c "me@host"    # new Ed25519 key
forged add work --file ~/.ssh/id_ed25519  # import existing
forged list                               # show all keys
forged list --json                        # machine-readable
forged export my-key                      # public key to stdout
forged rename my-key github               # rename
forged remove old-key                     # delete`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              To import keys from 1Password or your existing SSH agent:
            </p>
            <CodeBlock>{`forged migrate --from ssh          # import from ~/.ssh/
forged migrate --from 1password    # import from 1Password CLI
forged migrate --from agent        # list keys in current agent`}</CodeBlock>
          </Section>

          <Section id="host-matching" title="Host Matching">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Map keys to hosts so the right key is always offered first. Supports exact hostnames,
              wildcards, IP ranges, and regex.
            </p>
            <CodeBlock>{`forged host github "github.com" "*.github.com"
forged host deploy "*.prod.company.com" "10.0.*"
forged hosts                       # list all mappings
forged unhost deploy "10.0.*"      # remove a mapping`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              You can also configure host rules in <Code>~/.forged/config.toml</Code>:
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

          <Section id="git-signing" title="Git Signing">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Forged can sign your Git commits with SSH keys. Run <Code>forged setup</Code> and
              say yes to Git signing, or configure manually:
            </p>
            <CodeBlock title="~/.gitconfig">{`[user]
    signingkey = ssh-ed25519 AAAA...
[gpg]
    format = ssh
[gpg "ssh"]
    program = /usr/local/bin/forged-sign
[commit]
    gpgsign = true`}</CodeBlock>
          </Section>

          <Section id="sync" title="Cloud Sync">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Sync your encrypted vault across machines. The server stores only opaque blobs
              it cannot decrypt.
            </p>
            <CodeBlock>{`forged login                # opens browser for OAuth
forged sync                 # push/pull vault
forged sync status          # check sync state
forged logout               # clear credentials`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              On a new machine, install Forged, run <Code>forged login</Code> and <Code>forged sync</Code>,
              then enter your master password to decrypt the vault. All keys are available.
            </p>
          </Section>

          <Section id="configuration" title="Configuration">
            <p className="text-sm text-muted leading-relaxed mb-4">
              Configuration file location:
            </p>
            <ul className="text-sm text-muted space-y-1 mb-4">
              <li>macOS: <Code>~/.forged/config.toml</Code></li>
              <li>Linux: <Code>~/.config/forged/config.toml</Code></li>
            </ul>
            <CodeBlock title="config.toml">{`[agent]
socket = "~/.forged/agent.sock"
log_level = "info"

[sync]
enabled = false`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              Use <Code>forged config</Code> to open the config file in your editor,
              or <Code>forged config get</Code> / <Code>forged config set</Code> to manage individual values.
            </p>
          </Section>

          <Section id="commands" title="All Commands">
            <CodeBlock>{`forged setup                     first-time wizard
forged start / stop              manage daemon service
forged status                    daemon info + key count
forged doctor                    diagnose common issues

forged generate <name>           new Ed25519 key pair
forged add <name> --file <path>  import existing key
forged list                      all keys in vault
forged remove <name>             delete a key
forged export <name>             public key to stdout
forged rename <old> <new>        rename a key

forged host <key> <patterns>     map key to hosts
forged hosts                     list all mappings
forged unhost <key> <pattern>    remove a mapping

forged login                     authenticate with cloud
forged sync                      push/pull encrypted vault
forged sync status               check sync state
forged logout                    clear credentials

forged lock / unlock             clear or restore keys
forged change-password           change master password
forged migrate --from <source>   import from ssh/1password/agent
forged benchmark                 test Argon2id speed
forged logs                      tail daemon logs
forged config                    manage configuration`}</CodeBlock>
            <p className="text-sm text-muted leading-relaxed mt-4">
              All commands support <Code>--json</Code> for machine-readable output.
            </p>
          </Section>
        </main>
      </div>
    </>
  );
}
