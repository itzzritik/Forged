import Link from "next/link";

function ForgedMark() {
  return (
    <div className="relative w-8 h-8 flex items-center justify-center">
      <div className="absolute inset-0 rounded-md bg-white/10 blur-md" />
      <div className="relative w-8 h-8 rounded-md bg-white/5 border border-white/10 flex items-center justify-center">
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="text-white"
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
    <nav className="fixed top-0 left-0 right-0 z-50 border-b border-white/10 bg-black/50 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-6 h-14 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <ForgedMark />
          <span
            className="text-sm font-semibold tracking-widest text-white uppercase"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            forged
          </span>
        </div>
        <div className="flex items-center gap-6">
          <Link
            href="/docs"
            className="text-xs tracking-wider uppercase text-zinc-400 hover:text-white transition-colors"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            Docs
          </Link>
          <Link
            href="/security"
            className="text-xs tracking-wider uppercase text-zinc-400 hover:text-white transition-colors"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            Security
          </Link>
          <a
            href="https://github.com/itzzritik/forged"
            className="text-xs tracking-wider uppercase text-zinc-400 hover:text-white transition-colors"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            GitHub
          </a>
          <Link
            href="/login"
            className="text-xs uppercase tracking-wider text-black bg-white hover:bg-zinc-200 px-4 h-8 rounded shrink-0 flex items-center transition-colors font-bold"
            style={{ fontFamily: "var(--font-mono)" }}
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
    <section className="relative pt-40 pb-20 px-6 min-h-[85vh] flex flex-col justify-center overflow-hidden">
      {/* Background Graphic */}
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_rgba(255,255,255,0.05)_0%,_transparent_50%)] pointer-events-none" />
      <div
        className="absolute top-0 left-1/2 -translate-x-1/2 w-full max-w-4xl h-[1px] opacity-30"
        style={{
          background:
            "linear-gradient(90deg, transparent, rgba(255,255,255,0.8), transparent)",
        }}
      />
      <div className="absolute inset-0 bg-[url('data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjAiIGhlaWdodD0iNjAiIHZpZXdCb3g9IjAgMCA2MCA2MCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cGF0aCBkPSJNNTUgMGg1djYwaC01VjB6TTAgNTVoNjB2NUgwdi01eiIgZmlsbD0icmdiYSgyNTUsMjU1LDI1NSwwLjAyKSIgZmlsbC1ydWxlPSJldmVub2RkIiBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJub25lIi8+PC9zdmc+')] [mask-image:linear-gradient(to_bottom,white,transparent_80%)] pointer-events-none" />

      <div className="relative max-w-4xl mx-auto text-center z-10 w-full">
        <div className="inline-flex items-center gap-2 px-3 h-7 rounded border border-white/20 text-[10px] uppercase tracking-widest text-zinc-400 mb-8 bg-black/50 backdrop-blur-sm" style={{ fontFamily: "var(--font-mono)" }}>
          <span className="w-1.5 h-1.5 rounded-full bg-white animate-pulse" />
          The SSH App For Developers
        </div>
        <h1 className="text-5xl sm:text-7xl md:text-8xl font-bold tracking-tighter leading-[1.05] mb-8 text-white">
          Manage Keys.
          <br />
          <span className="text-zinc-600">Ship Faster.</span>
        </h1>
        <p className="text-lg md:text-xl text-zinc-400 max-w-2xl mx-auto mb-12 leading-relaxed">
          The fastest way to handle SSH keys. Open source, zero-knowledge, encrypted vault that syncs cross-platform automatically.
        </p>

        {/* Pseudo AI Prompt Input as CTA */}
        <div className="max-w-2xl mx-auto relative group">
          <div className="absolute -inset-1 bg-gradient-to-r from-white/20 to-white/0 rounded-xl blur opacity-25 group-hover:opacity-100 transition duration-1000 group-hover:duration-200" />
          <div className="relative flex flex-col sm:flex-row items-center p-2 rounded-xl bg-surface border border-white/20 shadow-2xl backdrop-blur-xl">
             <div className="flex-1 flex items-center justify-start px-4 h-14 w-full">
               <span className="text-zinc-500 mr-3 font-mono text-lg">$</span>
               <code className="text-white font-mono text-base tracking-wide whitespace-nowrap overflow-hidden pr-4 sm:pr-0">brew install forged</code>
             </div>
             <a
              href="https://github.com/itzzritik/forged"
              className="mt-2 sm:mt-0 w-full sm:w-auto h-12 px-8 bg-white text-black rounded-lg flex items-center justify-center text-sm font-bold tracking-wide uppercase hover:bg-zinc-200 transition-colors shrink-0"
              style={{ fontFamily: "var(--font-mono)" }}
            >
              Get Started
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}

function Problem() {
  const problems = [
    {
      icon: "M12 15v2m-6 4h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2zm10-10V7a4 4 0 0 0-8 0v4h8z",
      title: "Hard Drive Based",
      desc: "Your keys sit unprotected in ~/.ssh. Modern workflows require a master encrypted vault.",
    },
    {
      icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
      title: "Zero Sync",
      desc: "Moving from desktop to laptop means manually moving private key files. Stop doing that.",
    },
    {
      icon: "M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
      title: "Too Many Failures",
      desc: "SSH throws every key at the server until banned. Forged binds specific keys to specific hosts.",
    },
    {
      icon: "M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 1 1 3.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z",
      title: "Git Signing",
      desc: "A built in SSH agent allows frictionless, automatic verified signatures on git commits.",
    },
  ];

  return (
    <section className="py-24 px-6 border-t border-white/10 relative overflow-hidden bg-black">
      <div className="absolute left-0 top-0 bottom-0 w-px bg-white/10 hidden lg:block ml-6" />
      <div className="absolute right-0 top-0 bottom-0 w-px bg-white/10 hidden lg:block mr-6" />
      <div className="max-w-6xl mx-auto">
        <h2 className="text-3xl md:text-5xl font-bold tracking-tighter text-white mb-20 text-center">
          Fix the terminal.
        </h2>
        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-px bg-white/10 border border-white/10 rounded-xl overflow-hidden">
          {problems.map((p, i) => (
            <div
              key={p.title}
              className="p-8 bg-black hover:bg-zinc-900/50 transition-colors group"
            >
              <div className="w-10 h-10 rounded-lg bg-white/5 border border-white/10 flex items-center justify-center mb-6 group-hover:border-white/30 transition-colors">
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="text-white"
                >
                  <path d={p.icon} />
                </svg>
              </div>
              <h3 className="text-base font-semibold mb-3 text-white">{p.title}</h3>
              <p className="text-sm text-zinc-400 leading-relaxed">{p.desc}</p>
            </div>
          ))}
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
    <div className="rounded-xl bg-black border border-white/10 overflow-hidden relative group">
      <div className="absolute inset-0 bg-gradient-to-b from-white/5 to-transparent pointer-events-none opacity-0 group-hover:opacity-100 transition duration-500" />
      <div className="flex items-center justify-between px-4 h-10 border-b border-white/10 bg-white/5">
        <div className="flex gap-2">
          <div className="w-2.5 h-2.5 rounded-full bg-white/20 group-hover:bg-red-500/80 transition-colors" />
          <div className="w-2.5 h-2.5 rounded-full bg-white/20 group-hover:bg-yellow-500/80 transition-colors" />
          <div className="w-2.5 h-2.5 rounded-full bg-white/20 group-hover:bg-green-500/80 transition-colors" />
        </div>
        <span className="text-[10px] text-zinc-500 uppercase tracking-widest font-mono select-none">{title}</span>
      </div>
      <div
        className="p-5 text-[13px] leading-8 overflow-x-auto relative"
        style={{ fontFamily: "var(--font-mono)" }}
      >
        {lines.map((line, i) => (
          <div key={i} className="flex gap-3 whitespace-nowrap">
            {line.prompt && <span className="text-white/30 select-none">$</span>}
            {!line.prompt && <span className="text-white/0 select-none w-2 inline-block" />}
            <span className={line.prompt ? "text-white" : "text-zinc-500"}>
              {line.text}
            </span>
            {line.comment && (
              <span className="text-zinc-600 ml-4 hidden sm:inline-block"># {line.comment}</span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function HowItWorks() {
  return (
    <section className="py-24 px-6 border-t border-white/10 bg-black">
      <div className="max-w-6xl mx-auto flex flex-col lg:flex-row gap-16 items-center">
        <div className="flex-1 w-full">
          <div className="text-[10px] text-zinc-500 uppercase tracking-widest font-mono mb-4">Command Line Interface</div>
          <h2 className="text-3xl md:text-5xl font-bold tracking-tighter text-white mb-6">
            Instant execution.
          </h2>
          <p className="text-zinc-400 mb-8 max-w-md leading-relaxed text-lg">
            A single binary controls a daemon background service. Configure hosts, sync to cloud, and generate new keys instantly through forged.
          </p>
          <ul className="space-y-4 font-mono text-sm">
            {[
              "1. Download binary completely sandboxed",
              "2. Run setup mapped to local keys",
              "3. Bind custom hosts globally",
            ].map(item => (
              <li key={item} className="flex items-center text-zinc-500 gap-3">
                <span className="w-1.5 h-1.5 rounded-full bg-white"></span>
                {item}
              </li>
            ))}
          </ul>
        </div>
        <div className="flex-[1.2] w-full space-y-4">
          <CodeBlock
            title="Setup Engine"
            lines={[
              { prompt: true, text: "brew install forged", comment: "downloads ~15MB binary" },
              { prompt: true, text: "forged setup" },
              { prompt: false, text: "Master password: ********" },
              { prompt: false, text: "Importing 3 keys from ~/.ssh/" },
              { prompt: false, text: "Daemon initialized (PID 14930)" },
            ]}
          />
          <CodeBlock
            title="Intelligent Binding"
            lines={[
              {
                prompt: true,
                text: 'forged host github "*.github.com"',
              },
              { prompt: true, text: "forged list" },
              { prompt: false, text: "  github  *.github.com    (wildcard)" },
              {
                prompt: false,
                text: "  deploy  *.prod.company.com  (exact)",
              },
            ]}
          />
        </div>
      </div>
    </section>
  );
}

function Architecture() {
  const items = [
    {
      name: "Encrypted By Default",
      desc: "Argon2id + XChaCha20-Poly1305. The protocol standard for high risk key derivation.",
    },
    {
      name: "Unix Socket Agent",
      desc: "Emulates the exact ssh-agent protocol. Pure Go daemon, drops perfectly into any setup.",
    },
    {
      name: "Zero Knowledge Sync",
      desc: "Server architecture only stores heavily encrypted blobs. Vault is physically inaccessible.",
    },
  ];

  return (
    <section className="py-24 px-6 border-t border-white/10 bg-black overflow-hidden relative">
      <div className="max-w-6xl mx-auto">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold tracking-tighter text-white mb-6">
            Architecture
          </h2>
          <p className="text-zinc-400 max-w-2xl mx-auto text-lg">
            No Electron. No bloated browser extensions. Strictly terminal and background daemons written in modern Go. 
          </p>
        </div>
        
        <div className="grid md:grid-cols-3 gap-6">
            {items.map((item, i) => (
              <div
                key={item.name}
                className="p-8 rounded-xl bg-surface border border-white/10 flex flex-col justify-between hover:border-white/30 transition-colors"
              >
                <div>
                  <div className="text-[10px] text-zinc-500 uppercase tracking-widest font-mono mb-4">Module {i + 1}</div>
                  <h3 className="text-lg font-bold text-white mb-3 tracking-tight">{item.name}</h3>
                  <p className="text-sm text-zinc-400 leading-relaxed">{item.desc}</p>
                </div>
              </div>
            ))}
        </div>
      </div>
    </section>
  );
}

function Security() {
  return (
    <section className="py-32 px-6 border-y border-white/10 bg-black text-center relative overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(255,255,255,0.05)_0%,_transparent_50%)] pointer-events-none" />
      <div className="max-w-2xl mx-auto relative z-10">
        <h2 className="text-4xl md:text-6xl font-bold tracking-tighter text-white mb-8">
          Enterprise Security.
        </h2>
        <p className="text-zinc-400 text-lg md:text-xl leading-relaxed mb-12">
          Your keys never leave your machine decrypted. End-to-end atomic vault encryption synced directly between your clients. Fully auditable open-source core.
        </p>
        <Link
          href="/security"
          className="inline-flex h-12 px-8 bg-transparent text-white border border-white/30 rounded-lg items-center justify-center text-sm font-bold tracking-wide uppercase hover:bg-white hover:text-black transition-all"
          style={{ fontFamily: "var(--font-mono)" }}
        >
          Read Security Paper
        </Link>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="py-12 px-6 bg-black">
      <div className="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-6">
        <div className="flex items-center gap-3">
          <ForgedMark />
          <span
            className="text-xs uppercase font-bold tracking-widest text-zinc-500"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            forged inc {new Date().getFullYear()}
          </span>
        </div>
        <div className="flex items-center gap-8 text-xs font-mono uppercase tracking-widest text-zinc-500">
          <a
            href="https://github.com/itzzritik/forged"
            className="hover:text-white transition-colors"
          >
            GitHub
          </a>
          <Link
            href="/docs"
            className="hover:text-white transition-colors"
          >
            Docs
          </Link>
          <Link
            href="/security"
            className="hover:text-white transition-colors"
          >
             Privacy
          </Link>
        </div>
      </div>
    </footer>
  );
}

export default function Home() {
  return (
    <div className="bg-black mix-blend-normal">
      <Nav />
      <Hero />
      <Problem />
      <HowItWorks />
      <Architecture />
      <Security />
      <Footer />
    </div>
  );
}
