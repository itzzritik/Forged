import Link from "next/link";
import {
  ScrollReveal,
  GlitchButton,
  AnimatedTerminalGrid,
  AnimatedBigTerminal,
  TopologyVisualizer,
  TERMINAL_CARDS,
} from "@/components/client";
import type { TerminalStep } from "@/components/client";

function Nav() {
  return (
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
        <div className="flex items-center gap-6 md:gap-8">
          <Link href="/docs" className="hidden sm:block text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
            Docs
          </Link>
          <Link href="/security" className="hidden sm:block text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
            Security
          </Link>
          <a href="https://github.com/itzzritik/forged" className="hidden md:block text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
            GitHub
          </a>
          <GlitchButton href="/login" className="px-5 h-8 text-[12px]">Sign in</GlitchButton>
        </div>
      </div>
    </nav>
  );
}


const ROUTING_DEMO: TerminalStep[] = [
  {
    command: "forged status",
    output: [
      "Daemon: running (PID 4129)",
      "Keys:   3 loaded",
      "Socket: /Users/user/.forged/agent.sock",
    ],
    pauseAfter: 2000,
  },
  {
    command: "forged list",
    output: [
      "  NAME      TYPE         FINGERPRINT                                        SIGNING",
      "  --------  -----------  -------------------------------------------------  -------",
      "  github    ssh-ed25519  SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU  yes",
      "  deploy    ssh-ed25519  SHA256:jRkG8NpL2wQx5vB7tY1fH3sA9dK6uEiO4cX8rZ3bVnw",
      "  personal  ssh-rsa      SHA256:nVxK3mQR9f2QWv43kLwpQ2rBx87mN7pLq2iO8pK2wEzs",
    ],
    pauseAfter: 2500,
  },
  {
    command: 'forged host github "github.com" "*.github.com"',
    output: ["Mapped github to [github.com *.github.com]"],
    pauseAfter: 1800,
  },
  {
    command: 'forged host deploy "*.prod.company.com" "10.0.*"',
    output: ["Mapped deploy to [*.prod.company.com 10.0.*]"],
    pauseAfter: 2200,
  },
  {
    command: "forged hosts",
    output: [
      "  github\tgithub.com\t(exact)",
      "  github\t*.github.com\t(wildcard)",
      "  deploy\t*.prod.company.com\t(wildcard)",
      "  deploy\t10.0.*\t(wildcard)",
    ],
    pauseAfter: 2800,
  },
  {
    command: "ssh git@github.com",
    output: [
      "Hi user! You've successfully authenticated.",
      "Connection to github.com closed.",
    ],
    pauseAfter: 2200,
  },
  {
    command: "ssh deploy@api.prod.company.com",
    output: [
      "Welcome to Ubuntu 24.04.1 LTS (GNU/Linux 6.5.0-44-generic x86_64)",
      "",
      "Last login: Fri Apr 11 09:42:17 2026 from 10.0.1.5",
      "deploy@api-prod:~$",
    ],
    pauseAfter: 3000,
  },
];

function Hero() {
  return (
    <section className="relative h-dvh flex flex-col border-b border-[#27272a]">
      <section className="relative flex-1 overflow-hidden">
        {/* Terminal Grid Background */}
        <div className="absolute inset-0 pointer-events-none overflow-hidden select-none">
          <div className="h-full">
            <div className="relative h-full">
              <AnimatedTerminalGrid cards={TERMINAL_CARDS} />
              {/* Bottom gradient fade - 40% height, fully opaque at bottom */}
              <div className="pointer-events-none absolute bottom-0 left-0 right-0 h-[40%] bg-gradient-to-b from-transparent to-black" />
              {/* Left-side gradient for text readability */}
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-black via-black/80 via-40% to-transparent" />
              {/* Top edge fade */}
              <div className="pointer-events-none absolute top-0 left-0 right-0 h-20 bg-gradient-to-b from-black to-transparent" />
            </div>
          </div>
        </div>

        {/* Hero Content */}
        <div className="relative z-10 flex flex-col justify-center h-full px-6 lg:px-16">
          <div className="max-w-4xl">
            <div className="animate-slide-up">
              <div className="inline-flex items-center gap-2 px-3 py-1.5 bg-[#ea580c] text-black text-[11px] font-bold uppercase tracking-[0.15em] font-mono">
                ~/.ssh is a mess
              </div>
            </div>

            <h1 className="text-[clamp(48px,8vw,96px)] font-bold tracking-[-0.035em] leading-[0.9] text-white mt-8 animate-slide-up delay-100">
              Your Keys, <br />
              One Vault, <br />
              Every Machine.
            </h1>

            <p className="text-lg md:text-xl text-[#a1a1aa] max-w-2xl mt-8 mb-12 leading-relaxed animate-slide-up delay-200">
              Encrypt, sync, and manage all your SSH keys across every device.
              One command to install. One binary to run. You never touch ~/.ssh again.
            </p>

            <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4 animate-slide-up delay-300">
              <GlitchButton href="https://github.com/itzzritik/forged" external className="h-12 px-8">Install Forged</GlitchButton>
              <div className="flex items-center h-12 bg-[#09090b] border border-[#27272a] group hover:border-[#a1a1aa]/30 transition-colors">
                <span className="text-[#27272a] mx-3 font-mono text-lg select-none">$</span>
                <code className="text-white font-mono text-sm tracking-wide pr-4">
                  brew install forged
                </code>
                <div
                  className="w-12 h-full border-l border-[#27272a] flex items-center justify-center group-hover:bg-white/5 transition-colors cursor-pointer"
                  title="Copy"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-[#a1a1aa] group-hover:text-white transition-colors">
                    <rect x="9" y="9" width="13" height="13" />
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </section>
  );
}

function GridFeatures() {
  const features = [
    {
      icon: "M12 15v2m-6 4h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2zm10-10V7a4 4 0 0 0-8 0v4h8z",
      title: "Encrypted Vaults",
      subtitle: "Zero-knowledge storage",
      desc: "Your keys sit unprotected in ~/.ssh. Forged wraps every private key in an Argon2id + XChaCha20-Poly1305 vault that never decrypts on disk.",
      bullets: ["Argon2id key derivation", "XChaCha20-Poly1305 encryption", "mlock() memory protection"],
      cta: "Read Security Paper",
      href: "/security",
    },
    {
      icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
      title: "Cross-Platform Sync",
      subtitle: "Every device, one vault",
      desc: "Moving between machines means manually copying key files. Forged syncs encrypted blobs across all your devices automatically.",
      bullets: ["Zero-knowledge cloud sync", "HKDF-SHA256 derived sync keys", "Conflict-free propagation"],
      cta: "View Sync Docs",
      href: "/docs#sync",
    },
    {
      icon: "M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
      title: "Intelligent Binding",
      subtitle: "Pattern-matched routing",
      desc: "SSH throws every key at the server until banned. Forged binds specific keys to specific hosts using wildcard and regex patterns.",
      bullets: ["Wildcard host matching", "Regex pattern support", "Eliminates auth failures"],
      cta: "Configure Hosts",
      href: "/docs#host-matching",
    },
    {
      icon: "M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 1 1 3.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z",
      title: "Git Signatures",
      subtitle: "Verified commits",
      desc: "A built-in SSH agent allows frictionless, automatic verified signatures on every git commit across all your workflows.",
      bullets: ["Automatic commit signing", "SSH-based GPG alternative", "forged-sign helper binary"],
      cta: "Setup Signing",
      href: "/docs#git-signing",
    },
    {
      icon: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2H7a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2zM9 9h6v6H9V9z",
      title: "Unix Daemon",
      subtitle: "Always running, always ready",
      desc: "A single ~13MB Go binary runs a background daemon that emulates the ssh-agent protocol. No Electron, no browser extensions.",
      bullets: ["Pure Go socket agent", "launchctl/systemd binding", "0600 socket permissions"],
      cta: "View Architecture",
      href: "/docs#setup",
    },
    {
      icon: "M4 16l4.586-4.586a2 2 0 0 1 2.828 0L16 16m-2-2l1.586-1.586a2 2 0 0 1 2.828 0L20 14m-6-6h.01M6 20h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2z",
      title: "Key Migration",
      subtitle: "Import from anywhere",
      desc: "Migrate keys from ~/.ssh or 1Password in a single command. Inspect your running ssh-agent to plan the move.",
      bullets: ["Import from ~/.ssh", "1Password CLI integration", "Agent key discovery"],
      cta: "Migration Guide",
      href: "/docs#key-management",
    },
  ];

  return (
    <section className="relative py-36 px-6 lg:px-16 bg-black border-t border-white/10 overflow-hidden">
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <div className="relative z-10 w-full">
        <ScrollReveal className="mb-4">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">The SSH Platform</span>
          </div>
        </ScrollReveal>

        <ScrollReveal>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl xl:text-8xl font-bold tracking-tighter leading-[0.95] text-white text-pretty">
            Manage keys from anywhere, anytime, autonomously.
          </h2>
        </ScrollReveal>

        <ScrollReveal className="mt-6">
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed">
            One binary, six capabilities. Generate, encrypt, sync, bind, sign, and migrate your SSH keys from a single daemon - while you focus on shipping.
          </p>
        </ScrollReveal>

        <div className="relative z-10 mt-16 border-t border-l border-white/10 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => (
            <article
              key={f.title}
              className="group relative flex flex-col border-r border-b border-white/10 bg-white/[0.03] transition-colors duration-300 hover:border-[#ea580c]/20 cursor-pointer"
            >
              {/* Hover overlay */}
              <div className="absolute inset-0 bg-[#ea580c]/[0.07] pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity duration-300" />

              {/* Icon + Title */}
              <div className="relative flex items-center gap-3.5 px-6 pt-6 pb-3">
                <div className="flex items-center justify-center w-9 h-9 border bg-black border-white/10 text-white/70 group-hover:border-[#ea580c]/40 group-hover:text-orange-400 transition-colors duration-300">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                    <path d={f.icon} />
                  </svg>
                </div>
                <div>
                  <h3 className="text-sm font-semibold tracking-tight text-white group-hover:text-orange-400 transition-colors duration-300">{f.title}</h3>
                  <p className="text-xs text-[#a1a1aa] group-hover:text-orange-400/60 transition-colors duration-300">{f.subtitle}</p>
                </div>
              </div>

              {/* Description */}
              <p className="relative px-6 text-sm leading-relaxed text-white group-hover:text-orange-300 transition-colors duration-300">
                {f.desc}
              </p>

              {/* Bullets */}
              <ul className="relative px-6 pt-4 pb-2 flex-1 space-y-2">
                {f.bullets.map((b) => (
                  <li key={b} className="flex items-start gap-2.5 text-xs text-white group-hover:text-orange-300 transition-colors duration-300">
                    <span className="mt-1.5 h-1 w-1 bg-[#ea580c]/80 shrink-0" />
                    {b}
                  </li>
                ))}
              </ul>

              {/* CTA */}
              <div className="relative px-6 pb-6 pt-3 mt-auto">
                <Link
                  href={f.href}
                  className="inline-flex items-center gap-2 text-xs font-medium text-white group-hover:text-[#ea580c] transition-all duration-500 ease-in-out bg-[linear-gradient(to_right,rgba(234,88,12,0.4),rgba(234,88,12,0.4))] bg-[length:0%_1px] bg-[position:0%_100%] bg-no-repeat group-hover:bg-[length:100%_1px]"
                >
                  {f.cta}
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="opacity-0 group-hover:opacity-100 transition-opacity">
                    <line x1="5" y1="12" x2="19" y2="12" />
                    <polyline points="12 5 19 12 12 19" />
                  </svg>
                </Link>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function TerminalSection() {
  return (
    <section className="relative py-24 lg:py-36 bg-black border-t border-white/10 overflow-hidden">
      <div className="relative z-10 w-full px-6 lg:px-16 flex flex-col">
        {/* TOP COMPONENT - Chairman LLM Header Style */}
        <ScrollReveal className="w-full max-w-3xl mb-12 flex flex-col items-start text-left">
          <div className="mb-4 flex items-center gap-2.5">
             <span className="h-2 w-2 bg-[#ea580c]" />
             <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Pattern Matching</span>
          </div>
          <h2 className="text-5xl sm:text-6xl lg:text-7xl font-bold tracking-tighter leading-[0.95] text-white mb-6">
            Context Aware Routing.
          </h2>
          <p className="text-lg lg:text-xl text-[#a1a1aa] leading-relaxed mb-10 max-w-2xl">
            Never write another ~/.ssh/config file again. Forged uses wildcard and regex patterns to instantly route the correct cryptographic key to the right server, automatically.
          </p>
          <div className="flex flex-wrap items-center gap-4">
             <GlitchButton href="/docs" className="h-12 px-8 text-sm">Configure Patterns</GlitchButton>
             <GlitchButton href="/docs" variant="secondary" className="h-12 px-8 text-sm">View Docs</GlitchButton>
          </div>
        </ScrollReveal>

        {/* BOTTOM COMPONENT - Brutalist Data-Grid Terminal */}
        <ScrollReveal delay={0.2} className="w-full">
          <div className="border border-[#27272a] bg-[#050505] p-2 flex flex-col relative h-[500px] md:h-[600px] shadow-2xl overflow-hidden group">
            {/* Inner Screen Bezel */}
            <div className="border border-[#18181b] bg-black flex flex-col w-full h-full relative">
              {/* Mac-Style Header & Tab */}
              <div className="h-12 border-b border-[#18181b] bg-[#030303] flex items-center justify-between px-4 shrink-0 z-20">
                 <div className="flex items-center gap-4">
                   <div className="flex gap-2">
                     <div className="w-3 h-3 rounded-full bg-white/10 group-hover:bg-red-500/80 transition-colors" />
                     <div className="w-3 h-3 rounded-full bg-white/10 group-hover:bg-amber-500/80 transition-colors" />
                     <div className="w-3 h-3 rounded-full bg-white/10 group-hover:bg-emerald-500/80 transition-colors" />
                   </div>
                   
                   {/* Simple Path */}
                   <div className="h-4 w-px bg-[#27272a] mx-2" />
                   <span className="text-[#a1a1aa] font-mono text-[11px] tracking-widest uppercase mt-0.5">root@forged: ~</span>
                 </div>
                 
                 <div className="flex items-center gap-4">
                   <div className="flex items-center gap-2 border border-[#10b981]/30 bg-[#10b981]/10 px-2 py-1">
                     <span className="w-1.5 h-1.5 bg-[#10b981] animate-pulse" />
                     <span className="text-[9px] text-[#10b981] font-mono tracking-widest uppercase">ACTIVE</span>
                   </div>
                 </div>
              </div>
              
              {/* Terminal Body content */}
              <div className="flex-1 relative bg-black overflow-hidden">
                 <AnimatedBigTerminal steps={ROUTING_DEMO} />
              </div>
              
              {/* Data-Dense Footer */}
              <div className="h-8 border-t border-[#18181b] bg-[#050505] shrink-0 z-20 flex items-center justify-between px-4">
                 <div className="flex items-center gap-3">
                   <span className="text-[#a1a1aa] font-mono text-[9px] uppercase tracking-widest">MEM: 14.2MB</span>
                   <span className="text-[#a1a1aa] font-mono text-[9px] uppercase tracking-widest hidden sm:inline">| CPU: 0.1%</span>
                 </div>
                 <div className="flex items-center gap-1">
                   <span className="w-1.5 h-3 bg-[#ea580c]" />
                   <span className="w-1.5 h-3 bg-[#ea580c]" />
                   <span className="w-1.5 h-3 bg-[#ea580c]" />
                   <span className="w-1.5 h-3 bg-[#ea580c]/30" />
                   <span className="w-1.5 h-3 bg-[#ea580c]/30" />
                 </div>
              </div>
            </div>
          </div>
        </ScrollReveal>
      </div>
    </section>
  );
}

function Architecture() {
  const items = [
    {
      label: "Module 1",
      name: "Encrypted By Default",
      desc: "Argon2id + XChaCha20-Poly1305. The protocol standard for high risk key derivation.",
    },
    {
      label: "Module 2",
      name: "Unix Socket Agent",
      desc: "Emulates the exact ssh-agent protocol. Pure Go daemon, drops perfectly into any setup.",
    },
    {
      label: "Module 3",
      name: "Zero Knowledge Sync",
      desc: "Server architecture only stores heavily encrypted blobs. Vault is physically inaccessible.",
    },
  ];

  return (
    <section className="relative py-36 bg-black border-t border-white/10 overflow-hidden">
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <div className="relative z-10 w-full px-6 lg:px-16">
        <ScrollReveal className="mb-4">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Architecture</span>
          </div>
        </ScrollReveal>

        <ScrollReveal>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl xl:text-8xl font-bold tracking-tighter leading-[0.95] text-white text-pretty mb-6">
            Architecture
          </h2>
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed mb-16">
            No Electron. No bloated browser extensions. Strictly terminal and background daemons written in modern Go.
          </p>
        </ScrollReveal>

        <ScrollReveal>
          <TopologyVisualizer />
        </ScrollReveal>
      </div>
    </section>
  );
}

function EnterpriseSecurity() {
  const specs = [
    {
      id: "01",
      title: "CipherSuite",
      value: "XChaCha20",
      desc: "All vault data is encrypted using XChaCha20-Poly1305 AEAD. Extremely fast, deeply secure, and completely immune to timing attacks."
    },
    {
      id: "02",
      title: "Derivation",
      value: "Argon2id",
      desc: "Master keys are mathematically generated through Argon2id, the winner of the Password Hashing Competition. Highly ASIC resistant."
    },
    {
      id: "03",
      title: "Isolation",
      value: "M-Lock",
      desc: "The agent daemon uses unix.Mlock() to pin all decrypted memory pages, ensuring host OS swap-to-disk leaks are physically impossible."
    },
    {
      id: "04",
      title: "Auditability",
      value: "Open Core",
      desc: "The entire core daemon and CLI is open source. No proprietary telemetry, no opaque cryptographic implementations."
    }
  ];

  return (
    <section className="relative py-36 bg-black border-t border-[#27272a] overflow-hidden">
      {/* Brutalist Grid Background overlay */}
      <div className="absolute inset-0 opacity-[0.02]" style={{ backgroundImage: "linear-gradient(#fff 1px, transparent 1px), linear-gradient(90deg, #fff 1px, transparent 1px)", backgroundSize: "64px 64px" }} />

      <div className="relative z-10 w-full max-w-[1400px] mx-auto px-6 lg:px-16">
        
        {/* Deep industrial header */}
        <ScrollReveal className="mb-4">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Enterprise Security</span>
          </div>
        </ScrollReveal>

        <ScrollReveal>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl xl:text-8xl font-bold tracking-tighter leading-[0.95] text-white text-pretty mb-6">
            Zero Knowledge.
          </h2>
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed mb-16">
            We believe security through obscurity is no security at all. Forged is built entirely on open, mathematically auditable cryptographic standards. Your private keys never touch a disk unencrypted, and never leave your machine without end-to-end encryption.
          </p>
        </ScrollReveal>

        {/* The Specs Grid (Data-dense, purely typographical, massive impact) */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-px bg-[#27272a]">
           {specs.map((spec, i) => (
             <ScrollReveal key={spec.id} delay={i * 0.1} className="flex flex-col bg-black p-8 group relative overflow-hidden">
                 {/* Internal hover glow */}
                 <div className="absolute inset-0 bg-gradient-to-br from-[#ea580c]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
                 
                 <div className="flex items-center justify-between mb-8">
                     <span className="text-[#a1a1aa] font-mono text-xs tracking-widest uppercase">Spec // {spec.id}</span>
                     <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="text-[#3f3f46] group-hover:text-[#ea580c] transition-colors duration-500">
                        <path d="M5 12h14" />
                        <path d="M12 5l7 7-7 7" />
                     </svg>
                 </div>
                 <span className="text-sm font-mono tracking-widest uppercase text-[#ea580c] mb-1 opacity-80">{spec.title}</span>
                 <span className="text-3xl sm:text-4xl text-white font-bold tracking-tight mb-6">{spec.value}</span>
                 <p className="text-[#a1a1aa] text-sm leading-relaxed mt-auto relative z-10">
                   {spec.desc}
                 </p>
             </ScrollReveal>
           ))}
        </div>

        {/* Audit Badges / Trust center */}
        <ScrollReveal delay={0.2} className="mt-8 flex flex-col md:flex-row items-center justify-between border border-[#27272a] bg-[#020202] p-8 md:p-12 gap-8 relative overflow-hidden">
           {/* Scanline effect */}
           <div className="absolute inset-0 w-full h-[1px] bg-gradient-to-r from-transparent via-[#ea580c]/20 to-transparent animate-scan pointer-events-none" />
           
           <div className="flex-1 relative z-10">
             <h3 className="text-2xl md:text-3xl text-white font-bold mb-3 tracking-tight">Enterprise Ready. Fully Auditable.</h3>
             <p className="text-sm md:text-base text-[#a1a1aa] max-w-2xl">Read the complete cryptographic breakdown of our vault structure in the security whitepaper, or dive directly into the repository to audit the Go implementation yourself.</p>
           </div>
           
           <div className="flex flex-col sm:flex-row gap-4 w-full sm:w-auto relative z-10">
             <GlitchButton href="/security" className="h-14 px-8 border-[#ea580c] text-sm">Read Security Paper</GlitchButton>
             <GlitchButton href="https://github.com/itzzritik/forged" variant="secondary" className="h-14 px-8 text-sm">Audit Source Code</GlitchButton>
           </div>
        </ScrollReveal>

      </div>
    </section>
  );
}

function CTA() {
  return (
    <section className="relative py-36 bg-black border-t border-white/10 overflow-hidden text-center flex flex-col items-center justify-center">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(234,88,12,0.06)_0%,_transparent_60%)]" />
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <ScrollReveal className="relative z-10 w-full px-6 lg:px-16 max-w-4xl flex flex-col items-center">
        <div className="flex items-center gap-2.5 mb-6 justify-center">
          <span className="h-2 w-2 bg-[#ea580c] animate-pulse" />
          <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Get Started</span>
        </div>
        <h2 className="text-5xl sm:text-7xl lg:text-8xl xl:text-[100px] font-bold tracking-tighter leading-[0.9] text-white text-pretty mb-8">
          Secure your keys<br />Ship everything else
        </h2>
        <p className="text-base lg:text-xl text-[#a1a1aa] leading-relaxed mb-12 max-w-2xl">
          Install Forged. Never think about SSH key management again.
        </p>
        <div className="flex flex-col flex-wrap sm:flex-row items-center justify-center gap-6">
          <GlitchButton href="/login" className="h-14 px-12 text-sm max-w-full">Create Account</GlitchButton>
          <GlitchButton href="/docs" variant="secondary" className="h-14 px-12 text-sm max-w-full">Read Docs</GlitchButton>
        </div>
      </ScrollReveal>
    </section>
  );
}

function Footer() {
  return (
    <footer className="py-16 bg-black border-t border-[#27272a]">
      <div className="w-full px-6 lg:px-16 flex flex-col sm:flex-row items-center justify-between gap-6">
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

export default function Home() {
  return (
    <div className="bg-black">
      <Nav />
      <Hero />
      <GridFeatures />
      <TerminalSection />
      <Architecture />
      <EnterpriseSecurity />
      <CTA />
      <Footer />
    </div>
  );
}
