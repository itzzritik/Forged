import Link from "next/link";

function ForgedMark() {
  return (
    <div className="relative w-10 h-10 flex items-center justify-center">
      <div className="absolute inset-0 rounded-lg bg-gradient-to-br from-amber-500/20 to-orange-600/10 blur-xl" />
      <div className="relative w-10 h-10 rounded-lg bg-gradient-to-br from-amber-500/10 to-transparent border border-amber-500/20 flex items-center justify-center">
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="text-amber-400"
        >
          <path d="M15 3h6v6" />
          <path d="M10 14L21 3" />
          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        </svg>
      </div>
    </div>
  );
}

function Nav() {
  return (
    <nav className="w-full border-b border-border">
      <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <ForgedMark />
          <span
            className="text-sm font-medium tracking-tight"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            forged
          </span>
        </div>
        <div className="flex items-center gap-6">
          <Link
            href="/docs"
            className="text-sm text-muted hover:text-foreground transition-colors"
          >
            Docs
          </Link>
          <Link
            href="/security"
            className="text-sm text-muted hover:text-foreground transition-colors"
          >
            Security
          </Link>
          <a
            href="https://github.com/itzzritik/forged"
            className="text-sm text-muted hover:text-foreground transition-colors"
          >
            GitHub
          </a>
          <Link
            href="/login"
            className="text-sm text-zinc-900 bg-accent hover:bg-amber-400 px-4 h-8 rounded-md flex items-center transition-colors font-medium"
          >
            Sign in
          </Link>
        </div>
      </div>
    </nav>
  );
}

function Hero() {
  return (
    <section className="relative pt-24 pb-20 px-6 overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_rgba(245,158,11,0.04)_0%,_transparent_60%)]" />
      <div
        className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[1px]"
        style={{
          background:
            "linear-gradient(90deg, transparent, rgba(245,158,11,0.12), transparent)",
        }}
      />
      <div className="relative max-w-3xl mx-auto text-center">
        <div className="inline-flex items-center gap-2 px-3 h-7 rounded-full border border-border text-xs text-muted mb-8">
          <span className="w-1.5 h-1.5 rounded-full bg-amber-500" />
          Open source SSH key manager
        </div>
        <h1 className="text-4xl sm:text-5xl md:text-6xl font-medium tracking-tight leading-[1.1] mb-6">
          Forge your keys.
          <br />
          <span className="text-muted">Take them anywhere.</span>
        </h1>
        <p className="text-lg text-muted max-w-xl mx-auto mb-10 leading-relaxed">
          Encrypted vault, intelligent host matching, Git commit signing.
          A single binary that replaces 1Password and Bitwarden&apos;s SSH agent.
        </p>
        <div className="flex flex-col sm:flex-row gap-3 justify-center">
          <div
            className="inline-flex items-center gap-3 h-11 px-5 rounded-lg bg-surface border border-border text-sm"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            <span className="text-accent-dim">$</span>
            <span className="text-zinc-300">brew install forged</span>
          </div>
          <a
            href="https://github.com/itzzritik/forged"
            className="inline-flex items-center justify-center gap-2 h-11 px-5 rounded-lg bg-surface border border-border text-sm text-foreground hover:bg-surface-hover hover:border-border-hover transition-colors"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
            </svg>
            View on GitHub
          </a>
        </div>
      </div>
    </section>
  );
}

function Problem() {
  const problems = [
    {
      icon: "M12 15v2m-6 4h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2zm10-10V7a4 4 0 0 0-8 0v4h8z",
      title: "Unencrypted on disk",
      desc: "Your private keys sit in ~/.ssh/ as plain files. Anyone with laptop access has them.",
    },
    {
      icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
      title: "No sync between machines",
      desc: "Copy key files manually, or each machine has different keys. Neither is good.",
    },
    {
      icon: "M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
      title: "Wrong key, wrong host",
      desc: 'SSH tries every key until one works. You\'ve hit "too many authentication failures" before.',
    },
    {
      icon: "M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 1 1 3.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z",
      title: "Git signing is painful",
      desc: "A separate, manual setup that nobody finishes. Unsigned commits everywhere.",
    },
  ];

  return (
    <section className="py-20 px-6 border-t border-border">
      <div className="max-w-5xl mx-auto">
        <h2 className="text-2xl font-medium tracking-tight text-center mb-4">
          SSH keys deserve better
        </h2>
        <p className="text-muted text-center mb-14 max-w-lg mx-auto">
          The tools you use today were built for a simpler time.
        </p>
        <div className="grid sm:grid-cols-2 gap-6">
          {problems.map((p) => (
            <div
              key={p.title}
              className="p-6 rounded-xl bg-surface border border-border"
            >
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="text-accent-dim mb-4"
              >
                <path d={p.icon} />
              </svg>
              <h3 className="text-sm font-medium mb-2">{p.title}</h3>
              <p className="text-sm text-muted leading-relaxed">{p.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function HowItWorks() {
  return (
    <section className="py-20 px-6 border-t border-border">
      <div className="max-w-5xl mx-auto">
        <h2 className="text-2xl font-medium tracking-tight text-center mb-4">
          One command to start
        </h2>
        <p className="text-muted text-center mb-14 max-w-lg mx-auto">
          Setup takes 30 seconds. Then SSH and Git just work.
        </p>
        <div className="max-w-2xl mx-auto space-y-4">
          <CodeBlock
            title="Setup"
            lines={[
              { prompt: true, text: "brew install forged" },
              { prompt: true, text: "forged setup" },
              { prompt: false, text: "Create a master password: ********" },
              { prompt: false, text: "Imported 3 keys from ~/.ssh/" },
              { prompt: false, text: "Daemon running (PID 12345)" },
              { prompt: false, text: "Setup complete!" },
            ]}
          />
          <CodeBlock
            title="Daily use"
            lines={[
              { prompt: true, text: "ssh myserver", comment: "right key, automatically" },
              { prompt: true, text: "git push origin main", comment: "commits signed" },
              { prompt: true, text: "forged list" },
              {
                prompt: false,
                text: "  github    ssh-ed25519  SHA256:abc...",
              },
              {
                prompt: false,
                text: "  deploy    ssh-ed25519  SHA256:def...",
              },
            ]}
          />
          <CodeBlock
            title="Host matching"
            lines={[
              {
                prompt: true,
                text: 'forged host github "github.com" "*.github.com"',
              },
              {
                prompt: true,
                text: 'forged host deploy "*.prod.company.com"',
              },
              { prompt: true, text: "forged hosts" },
              { prompt: false, text: "  github  github.com      (exact)" },
              { prompt: false, text: "  github  *.github.com    (wildcard)" },
              {
                prompt: false,
                text: "  deploy  *.prod.company.com  (wildcard)",
              },
            ]}
          />
        </div>
      </div>
    </section>
  );
}

function CodeBlock({
  title,
  lines,
}: {
  title: string;
  lines: { prompt: boolean; text: string; comment?: string }[];
}) {
  return (
    <div className="rounded-xl bg-surface border border-border overflow-hidden">
      <div className="flex items-center gap-2 px-4 h-9 border-b border-border">
        <div className="flex gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
          <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
          <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
        </div>
        <span className="text-xs text-muted ml-2">{title}</span>
      </div>
      <div
        className="p-4 text-sm leading-7 overflow-x-auto"
        style={{ fontFamily: "var(--font-mono)" }}
      >
        {lines.map((line, i) => (
          <div key={i} className="flex gap-2">
            {line.prompt && <span className="text-accent-dim select-none">$</span>}
            <span className={line.prompt ? "text-zinc-200" : "text-zinc-500"}>
              {line.text}
            </span>
            {line.comment && (
              <span className="text-zinc-600 ml-2"># {line.comment}</span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function Architecture() {
  const items = [
    {
      name: "SSH Agent",
      desc: "Standard protocol. ssh-add works, any SSH client works.",
    },
    {
      name: "Encrypted Vault",
      desc: "Argon2id + XChaCha20-Poly1305. Atomic writes.",
    },
    {
      name: "Host Matcher",
      desc: "Right key for each host, automatically.",
    },
    {
      name: "Key Store",
      desc: "In-memory, mlock'd, zeroed on shutdown.",
    },
    {
      name: "Cloud Sync",
      desc: "Zero-knowledge. Server stores opaque blobs.",
    },
  ];

  return (
    <section className="py-20 px-6 border-t border-border">
      <div className="max-w-5xl mx-auto">
        <h2 className="text-2xl font-medium tracking-tight text-center mb-4">
          How it works
        </h2>
        <p className="text-muted text-center mb-14 max-w-lg mx-auto">
          A background daemon that speaks the standard SSH agent protocol.
          No browser, no Electron. Just a Unix socket and a CLI.
        </p>
        <div className="max-w-2xl mx-auto">
          <div className="space-y-3">
            {items.map((item, i) => (
              <div
                key={item.name}
                className="flex items-start gap-4 p-4 rounded-xl bg-surface border border-border"
              >
                <div className="w-6 h-6 rounded-md bg-accent/10 border border-accent/20 flex items-center justify-center text-xs text-accent shrink-0 mt-0.5">
                  {i + 1}
                </div>
                <div>
                  <h3 className="text-sm font-medium mb-1">{item.name}</h3>
                  <p className="text-sm text-muted">{item.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function Comparison() {
  const headers = ["", "Forged", "1Password", "Bitwarden", "Secretive", "ssh-agent"];
  const rows = [
    ["Standalone", "Yes", "No", "No", "Yes", "Yes"],
    ["Cross-platform", "Mac/Linux/Win", "Mac/Linux/Win", "Mac/Linux/Win", "Mac only", "Mac/Linux"],
    ["Key sync", "Yes", "Bundled", "Bundled", "No", "No"],
    ["Host matching", "Smart", "Basic", "No", "No", "No"],
    ["Git signing", "Built-in", "Yes", "No", "Yes", "Manual"],
    ["Auth model", "Login once", "Per use", "Per use", "Per use", "Per session"],
    ["Open source", "Yes", "No", "Yes", "Yes", "Yes"],
  ];

  return (
    <section className="py-20 px-6 border-t border-border">
      <div className="max-w-5xl mx-auto">
        <h2 className="text-2xl font-medium tracking-tight text-center mb-14">
          Comparison
        </h2>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr>
                {headers.map((h, i) => (
                  <th
                    key={h || "feature"}
                    className={`pb-4 text-left font-medium ${
                      i === 0 ? "text-muted" : i === 1 ? "text-accent" : "text-muted"
                    }`}
                    style={i === 0 ? {} : { fontFamily: "var(--font-mono)" }}
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row[0]} className="border-t border-border">
                  {row.map((cell, i) => (
                    <td
                      key={`${row[0]}-${i}`}
                      className={`py-3 pr-6 ${
                        i === 0
                          ? "text-foreground font-medium"
                          : i === 1
                          ? "text-zinc-200"
                          : "text-muted"
                      }`}
                    >
                      {cell === "Yes" && i === 1 ? (
                        <span className="text-accent">{cell}</span>
                      ) : cell === "No" ? (
                        <span className="text-zinc-600">{cell}</span>
                      ) : (
                        cell
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function Security() {
  return (
    <section className="py-20 px-6 border-t border-border">
      <div className="max-w-3xl mx-auto text-center">
        <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-accent/10 border border-accent/20 mb-6">
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="text-accent"
          >
            <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
        </div>
        <h2 className="text-2xl font-medium tracking-tight mb-4">
          Zero-knowledge security
        </h2>
        <p className="text-muted leading-relaxed mb-8 max-w-xl mx-auto">
          Your master password never leaves your machine. The server stores
          opaque encrypted blobs. It cannot decrypt your vault, read your keys,
          or see your master password. The same architecture used by 1Password
          and Bitwarden.
        </p>
        <div className="grid sm:grid-cols-3 gap-4 text-left">
          {[
            {
              label: "Encryption",
              value: "Argon2id + XChaCha20-Poly1305",
            },
            {
              label: "Memory",
              value: "mlock'd pages, zeroed on shutdown",
            },
            {
              label: "Vault",
              value: "Atomic writes, flock, 0600 permissions",
            },
          ].map((item) => (
            <div
              key={item.label}
              className="p-4 rounded-xl bg-surface border border-border"
            >
              <div className="text-xs text-accent-dim mb-1">{item.label}</div>
              <div className="text-sm text-zinc-300">{item.value}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function CTA() {
  return (
    <section className="py-20 px-6 border-t border-border">
      <div className="max-w-3xl mx-auto text-center">
        <h2 className="text-3xl font-medium tracking-tight mb-4">
          Ready to forge your keys?
        </h2>
        <p className="text-muted mb-8">
          One command. 30 seconds. Your SSH keys will thank you.
        </p>
        <div
          className="inline-flex items-center gap-3 h-12 px-6 rounded-lg bg-surface border border-border text-sm"
          style={{ fontFamily: "var(--font-mono)" }}
        >
          <span className="text-accent-dim">$</span>
          <span className="text-zinc-300">brew install forged</span>
        </div>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="border-t border-border py-8 px-6">
      <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <ForgedMark />
          <span
            className="text-sm text-muted"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            forged
          </span>
        </div>
        <div className="flex items-center gap-6 text-sm text-muted">
          <a
            href="https://github.com/itzzritik/forged"
            className="hover:text-foreground transition-colors"
          >
            GitHub
          </a>
          <Link
            href="/login"
            className="hover:text-foreground transition-colors"
          >
            Sign in
          </Link>
        </div>
      </div>
    </footer>
  );
}

export default function Home() {
  return (
    <>
      <Nav />
      <Hero />
      <Problem />
      <HowItWorks />
      <Architecture />
      <Comparison />
      <Security />
      <CTA />
      <Footer />
    </>
  );
}
