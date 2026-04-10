import Link from "next/link";
import {
  ScrollReveal,
  StaggerGrid,
  StaggerItem,
  SpotlightCard,
  GlitchButton,
} from "@/components/client";

function Nav() {
  return (
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

const TERMINAL_CARDS: { title: string; status: "green" | "green-pulse" | "amber" | "amber-pulse" | "red-pulse"; brightness: number; lines: string[] }[] = [
  { title: "SETUP // BOOTSTRAP", status: "green-pulse", brightness: 1.0, lines: [
    "> forged setup",
    "[INIT] Creating vault at ~/.forged/",
    "[INIT] Master password: ********",
    "[SCAN] Found 5 keys in ~/.ssh/",
    "[IMPORT] id_ed25519 ... ok",
    "[IMPORT] id_rsa ... ok",
    "[IMPORT] deploy_key ... ok",
    "[DAEMON] Binding socket 0600",
  ]},
  { title: "AGENT // STATUS", status: "green", brightness: 0.75, lines: [
    "> forged status",
    "daemon:  running (pid 4821)",
    "socket:  /tmp/forged.sock",
    "keys:    4 loaded",
    "hosts:   3 bound",
    "uptime:  14h 23m 07s",
    "memory:  2.1 MB resident",
  ]},
  { title: "KEYGEN // ED25519", status: "green-pulse", brightness: 1.1, lines: [
    "> forged generate deploy-prod",
    "[GEN] Algorithm: Ed25519",
    "[GEN] Comment: deploy@prod",
    "[VAULT] Encrypting with XChaCha20",
    "[VAULT] Nonce: random 24-byte",
    "[OK] Key deploy-prod created",
    "[OK] Vault synced to disk",
  ]},
  { title: "SYNC // CLOUD", status: "green", brightness: 1.1, lines: [
    "> forged sync",
    "[AUTH] Token valid (exp 2026-04-11)",
    "[HKDF] Deriving sync key...",
    "[UPLOAD] Encrypting vault blob",
    "[UPLOAD] 4.2 KB -> blob storage",
    "[OK] Sync complete (312ms)",
    "[OK] 4 keys propagated",
  ]},
  { title: "HOST // BINDING", status: "green", brightness: 0.75, lines: [
    "> forged hosts",
    "  NAME        PATTERNS",
    "  github      *.github.com",
    "  deploy      *.prod.company.com",
    "  personal    *",
    "> forged host github \"github.com\"",
    "[OK] Pattern bound: github.com",
  ]},
  { title: "SSH // CONNECT", status: "green-pulse", brightness: 1.1, lines: [
    "> ssh git@github.com",
    "[AGENT] Request from ssh (pid 9102)",
    "[MATCH] github.com -> github key",
    "[AUTH] Ed25519 challenge-response",
    "[OK] Authenticated as git",
    "Hi user! You've successfully",
    "authenticated with key: github",
  ]},
  { title: "MIGRATE // IMPORT", status: "amber", brightness: 1.0, lines: [
    "> forged migrate --from ssh",
    "[SCAN] Reading ~/.ssh/ ...",
    "[FOUND] id_ed25519 (4096 bit)",
    "[FOUND] id_rsa (2048 bit)",
    "[WARN] id_rsa uses weak RSA-2048",
    "[IMPORT] 2 keys ingested",
    "[VAULT] Re-encrypted with Argon2id",
  ]},
  { title: "GIT // SIGNING", status: "green", brightness: 0.75, lines: [
    "> git commit -m \"fix auth flow\"",
    "[SIGN] Request from git (pid 3401)",
    "[MATCH] git -> signing key",
    "[SIGN] SSH signature created",
    "[OK] Commit signed: a3f2b1c",
    "[OK] Verified: ssh-ed25519",
    "1 file changed, 12 insertions(+)",
  ]},
  { title: "VAULT // ENCRYPT", status: "green", brightness: 0.75, lines: [
    "> forged lock",
    "[LOCK] Zeroing memory pages...",
    "[LOCK] mlock() released 4 pages",
    "[LOCK] Agent socket suspended",
    "[OK] Vault locked, 0 keys in mem",
    "> forged unlock",
    "Master password: ********",
  ]},
  { title: "DAEMON // LOGS", status: "green-pulse", brightness: 1.0, lines: [
    "> forged logs --tail",
    "14:23:07 [INFO] github.com -> ok",
    "14:23:08 [INFO] key: github",
    "14:23:08 [INFO] auth: success",
    "14:23:09 [INFO] session: active",
    "14:25:11 [INFO] prod.co -> ok",
    "14:25:12 [INFO] key: deploy",
  ]},
  { title: "CONFIG // TOML", status: "green", brightness: 1.1, lines: [
    "[[hosts]]",
    "name = \"GitHub\"",
    "match = [\"github.com\"]",
    "key = \"github\"",
    "git_signing = true",
    "",
    "[[hosts]]",
  ]},
  { title: "DOCTOR // CHECK", status: "green-pulse", brightness: 1.0, lines: [
    "> forged doctor",
    "[CHECK] Vault integrity ... ok",
    "[CHECK] Daemon running ... ok",
    "[CHECK] Socket perms 0600 ... ok",
    "[CHECK] SSH config ... ok",
    "[CHECK] Argon2id params ... ok",
    "[OK] All 5 checks passed",
  ]},
  { title: "LIST // KEYS", status: "green", brightness: 0.75, lines: [
    "> forged list",
    "  NAME       TYPE      CREATED",
    "  github     ed25519   2025-03-14",
    "  deploy     ed25519   2025-06-01",
    "  personal   rsa-4096  2024-11-22",
    "  signing    ed25519   2025-01-09",
    "4 keys in vault",
  ]},
  { title: "EXPORT // KEY", status: "green", brightness: 1.1, lines: [
    "> forged export github --pub",
    "ssh-ed25519 AAAAC3NzaC1lZDI1",
    "NTE5AAAAIG8f3kR7vKJzMnL+hW2",
    "Kf9mN3pQ5xR1tY6uI0oP8aS2dF4",
    "gH7jK1lZ3xC5vB9nM2qW8eR6tY0",
    "uI4oP7aS1dF3gH6jK0 github@f",
    "[OK] Public key written to stdout",
  ]},
  { title: "BENCHMARK // ARGON", status: "amber-pulse", brightness: 1.0, lines: [
    "> forged benchmark",
    "[BENCH] Argon2id 64MB 3 iter",
    "[BENCH] Derive: 287ms avg",
    "[BENCH] Encrypt: 0.4ms avg",
    "[BENCH] Decrypt: 0.3ms avg",
    "[BENCH] Total: 288ms per unlock",
    "[OK] Within security threshold",
  ]},
  { title: "SYNC // STATUS", status: "green", brightness: 1.1, lines: [
    "> forged sync status",
    "account:  user@forged.dev",
    "last_sync: 2m ago",
    "blob_size: 4.2 KB",
    "devices:   3 linked",
    "conflicts: 0",
    "[OK] Vault in sync",
  ]},
  { title: "SECURITY // AUDIT", status: "red-pulse", brightness: 1.0, lines: [
    "> forged audit",
    "[AUDIT] Checking key strength...",
    "[OK] github: ed25519 (strong)",
    "[OK] deploy: ed25519 (strong)",
    "[WARN] personal: rsa-2048 (weak)",
    "[OK] signing: ed25519 (strong)",
    "[WARN] 1 key below threshold",
  ]},
  { title: "RENAME // KEY", status: "green", brightness: 0.75, lines: [
    "> forged rename personal backup",
    "[VAULT] Updating identifier...",
    "[VAULT] Re-encrypting entry...",
    "[OK] personal -> backup",
    "> forged list --json",
    "{\"keys\":[{\"name\":\"backup\",",
    "\"type\":\"rsa\",\"bits\":4096}]}",
  ]},
  { title: "UNHOST // UNBIND", status: "green", brightness: 1.1, lines: [
    "> forged unhost deploy \"10.0.*\"",
    "[ROUTE] Removing pattern...",
    "[OK] Unbound: 10.0.*",
    "> forged hosts",
    "  NAME     PATTERNS",
    "  github   *.github.com",
    "  deploy   *.prod.company.com",
  ]},
  { title: "DAEMON // START", status: "green-pulse", brightness: 1.0, lines: [
    "> forged start --background",
    "[DAEMON] Forking to background...",
    "[DAEMON] PID 4821 registered",
    "[SOCKET] Listening on agent.sock",
    "[SOCKET] Permissions: 0600",
    "[VAULT] 4 keys loaded to memory",
    "[OK] Agent ready, accepting conn",
  ]},
];

function TerminalCard({ title, status, brightness, lines }: typeof TERMINAL_CARDS[number]) {
  const dotColor = {
    green: "bg-emerald-500/80",
    "green-pulse": "bg-emerald-400 animate-pulse",
    amber: "bg-amber-500/80",
    "amber-pulse": "bg-amber-400 animate-pulse",
    "red-pulse": "bg-red-400 animate-pulse",
  }[status];

  return (
    <div
      className="flex flex-col bg-black border border-[#1a1a1a] overflow-hidden"
      style={{ filter: `brightness(${brightness})` }}
    >
      <div className="flex items-center justify-between h-7 px-3 border-b border-[#1a1a1a] shrink-0">
        <span className="text-[10px] text-[#555] font-mono tracking-[0.5px]">{title}</span>
        <span className={`w-1.5 h-1.5 rounded-full ${dotColor}`} />
      </div>
      <div className="flex-1 p-3 overflow-hidden">
        {lines.map((line, i) => (
          <div key={i} className="font-mono text-[11px] leading-[1.7] whitespace-pre truncate" style={{ color: line.startsWith(">") ? "#999" : line.includes("[WARN]") ? "rgb(251 191 36 / 0.8)" : line.includes("[ERR]") || line.includes("[WARN] 1") ? "rgb(248 113 113 / 0.8)" : "#666" }}>
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}

function Hero() {
  return (
    <section className="relative h-dvh flex flex-col border-b border-[#27272a]">
      <section className="relative flex-1 overflow-hidden">
        {/* Terminal Grid Background */}
        <div className="absolute inset-0 pointer-events-none overflow-hidden select-none">
          <div className="h-full">
            <div className="relative h-full">
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 grid-rows-5 gap-px bg-[#1a1a1a] h-full">
                {TERMINAL_CARDS.map((card) => (
                  <TerminalCard key={card.title} {...card} />
                ))}
              </div>
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
      desc: "Your keys sit unprotected in ~/.ssh. Modern workflows require a master encrypted vault. Argon2id derived keys ensure absolute security.",
    },
    {
      icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
      title: "Cross-Platform Sync",
      desc: "Moving from desktop to laptop means manually moving private key files. Synchronize cryptographically locked payloads automatically.",
    },
    {
      icon: "M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
      title: "Intelligent Binding",
      desc: "SSH throws every key at the server until banned. Forged allows you to bind specific keys to specific hosts using pattern matching.",
    },
    {
      icon: "M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 1 1 3.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z",
      title: "Git Signatures",
      desc: "An internally built SSH agent allows seamless, automatic verified signatures on git commits across all your workflows.",
    },
  ];

  return (
    <section className="py-32 px-6 bg-black border-b border-[#27272a]">
      <div className="max-w-7xl mx-auto">
        <ScrollReveal>
          <h2 className="text-4xl md:text-6xl font-bold tracking-tighter text-white mb-20 max-w-3xl leading-none">
            Manage keys from anywhere, anytime, autonomously.
          </h2>
        </ScrollReveal>

        <StaggerGrid className="grid md:grid-cols-2 lg:grid-cols-4 gap-px bg-[#27272a]" stagger={0.1}>
          {features.map((f) => (
            <StaggerItem key={f.title} className="bg-[#09090b]">
              <SpotlightCard className="h-full group hover:bg-[#18181b] transition-colors duration-300">
                <div className="h-px bg-[#ea580c] scale-x-0 group-hover:scale-x-100 transition-transform origin-left duration-500 ease-out" />
                <div className="px-8 py-12">
                  <div className="w-12 h-12 bg-[#09090b] border border-[#27272a] flex items-center justify-center mb-8 group-hover:border-[#ea580c]/40 group-hover:bg-[#ea580c]/5 transition-all duration-300">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="text-[#a1a1aa] group-hover:text-[#ea580c] transition-colors">
                      <path d={f.icon} />
                    </svg>
                  </div>
                  <h3 className="text-lg font-bold mb-4 text-white group-hover:text-[#ea580c] transition-colors">
                    {f.title}
                  </h3>
                  <p className="text-sm text-[#a1a1aa] leading-relaxed">{f.desc}</p>
                </div>
              </SpotlightCard>
            </StaggerItem>
          ))}
        </StaggerGrid>
      </div>
    </section>
  );
}

function TerminalSection() {
  return (
    <section className="py-32 px-6 border-b border-[#27272a] bg-black relative">
      <div className="absolute inset-0 mesh-texture opacity-30 pointer-events-none" />

      <div className="max-w-7xl mx-auto flex flex-col lg:flex-row gap-20 items-center relative z-10">
        <ScrollReveal className="flex-1 w-full">
          <div className="text-[10px] text-[#ea580c] uppercase tracking-widest font-mono mb-4 px-2 py-1 border border-[#ea580c]/20 bg-[#ea580c]/5 inline-block">
            Background Daemon
          </div>
          <h2 className="text-4xl md:text-6xl font-bold tracking-tighter text-white mb-6 leading-none">
            Execute in Parallel.
          </h2>
          <p className="text-[#a1a1aa] mb-8 max-w-lg leading-relaxed text-lg">
            A single 15MB binary controls a robust daemon service. Configure
            patterns, sync to the cloud, and orchestrate instances instantly
            without modifying ~/.ssh.
          </p>
          <ul className="space-y-5 font-mono text-[13px]">
            {[
              "Completely sandboxed execution",
              "Pure Go Socket Agent emulator",
              "Pattern matched host binding",
            ].map((item) => (
              <li key={item} className="flex items-center text-[#a1a1aa] gap-4 group">
                <span className="w-2 h-2 bg-[#27272a] group-hover:bg-[#ea580c] group-hover:animate-pulse transition-colors" />
                <span className="group-hover:text-white transition-colors">{item}</span>
              </li>
            ))}
          </ul>
        </ScrollReveal>

        <ScrollReveal delay={0.2} className="flex-[1.2] w-full">
          <div className="border border-[#27272a] bg-black overflow-hidden shadow-2xl relative group">
            <div className="flex items-center justify-between px-4 h-11 border-b border-[#27272a] bg-[#09090b]">
              <div className="flex items-center gap-6">
                <div className="flex gap-2">
                  <div className="w-3 h-3 rounded-full bg-[#27272a] group-hover:bg-[#FF5F56] transition-colors duration-300" />
                  <div className="w-3 h-3 rounded-full bg-[#27272a] group-hover:bg-[#FFBD2E] transition-colors duration-300" />
                  <div className="w-3 h-3 rounded-full bg-[#27272a] group-hover:bg-[#27C93F] transition-colors duration-300" />
                </div>
                <div className="flex items-center gap-4 text-xs font-mono text-[#a1a1aa]">
                  <span className="text-white border-b border-[#ea580c] pb-2.5 translate-y-[6px]">config.toml</span>
                  <span className="hover:text-white transition-colors cursor-pointer">daemon.go</span>
                </div>
              </div>
              <span className="text-[10px] text-[#27272a] uppercase tracking-widest font-mono select-none">
                Unix Socket
              </span>
            </div>

            <div className="p-6 overflow-x-auto relative min-h-[300px]">
              <div className="absolute left-0 top-0 bottom-0 w-12 border-r border-[#27272a]/50 bg-black/50 flex flex-col items-center py-6 text-[10px] text-[#27272a] font-mono select-none">
                {Array.from({ length: 12 }).map((_, i) => (
                  <span key={i} className="leading-7">{i + 1}</span>
                ))}
              </div>

              <div className="ml-8 font-mono text-[13px] leading-7">
                <div className="text-[#a1a1aa]"># Forged Infrastructure</div>
                <div>
                  <span className="text-[#ea580c]">[[</span>
                  <span className="text-white">hosts</span>
                  <span className="text-[#ea580c]">]]</span>
                </div>
                <div>
                  <span className="text-[#00bcd4]">name</span>{" "}
                  <span className="text-[#a1a1aa]">=</span>{" "}
                  <span className="text-emerald-500">&quot;Production AWS&quot;</span>
                </div>
                <div>
                  <span className="text-[#00bcd4]">match</span>{" "}
                  <span className="text-[#a1a1aa]">=</span>{" "}
                  <span className="text-white">[&quot;*.prod.company.com&quot;, &quot;10.0.*&quot;]</span>
                </div>
                <div>
                  <span className="text-[#00bcd4]">key</span>{" "}
                  <span className="text-[#a1a1aa]">=</span>{" "}
                  <span className="text-emerald-500">&quot;deploy_key_ed25519&quot;</span>
                </div>
                <div className="h-7" />
                <div className="text-[#a1a1aa]"># Run the daemon</div>
                <div className="flex items-center gap-3 bg-[#09090b] border border-[#27272a] p-3 group-hover:border-[#ea580c]/40 transition-colors w-max mt-2">
                  <span className="text-[#27272a]">$</span>
                  <span className="text-white">forged start --background</span>
                </div>
              </div>
              <div className="absolute bottom-0 left-0 right-0 h-24 bg-gradient-to-t from-black via-black/80 to-transparent pointer-events-none" />
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
    <section className="py-32 px-6 border-b border-[#27272a] bg-black">
      <div className="max-w-7xl mx-auto">
        <ScrollReveal className="text-center mb-20">
          <h2 className="text-4xl md:text-6xl font-bold tracking-tighter text-white mb-6 leading-none">
            Architecture
          </h2>
          <p className="text-[#a1a1aa] max-w-2xl mx-auto text-lg">
            No Electron. No bloated browser extensions. Strictly terminal and background daemons written in modern Go.
          </p>
        </ScrollReveal>

        <StaggerGrid className="grid md:grid-cols-3 gap-px bg-[#27272a]" stagger={0.12}>
          {items.map((item) => (
            <StaggerItem key={item.name} className="bg-[#09090b]">
              <SpotlightCard className="h-full group hover:bg-[#18181b] transition-colors duration-300">
                <div className="h-px bg-[#ea580c] scale-x-0 group-hover:scale-x-100 transition-transform origin-left duration-500 ease-out" />
                <div className="p-8">
                  <div className="text-[10px] text-[#a1a1aa] uppercase tracking-widest font-mono mb-4">
                    {item.label}
                  </div>
                  <h3 className="text-lg font-bold text-white mb-3 tracking-tight group-hover:text-[#ea580c] transition-colors">
                    {item.name}
                  </h3>
                  <p className="text-sm text-[#a1a1aa] leading-relaxed">{item.desc}</p>
                </div>
              </SpotlightCard>
            </StaggerItem>
          ))}
        </StaggerGrid>
      </div>
    </section>
  );
}

function SecurityCTA() {
  return (
    <section className="py-32 px-6 border-b border-[#27272a] bg-black text-center relative overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(255,255,255,0.03)_0%,_transparent_50%)] pointer-events-none" />
      <ScrollReveal className="max-w-2xl mx-auto relative z-10">
        <h2 className="text-4xl md:text-6xl font-bold tracking-tighter text-white mb-8 leading-none">
          Enterprise Security.
        </h2>
        <p className="text-[#a1a1aa] text-lg md:text-xl leading-relaxed mb-12">
          Your keys never leave your machine decrypted. End-to-end atomic vault encryption synced directly between your clients. Fully auditable open-source core.
        </p>
        <GlitchButton href="/security" variant="secondary" className="h-12 px-10">Read Security Paper</GlitchButton>
      </ScrollReveal>
    </section>
  );
}

function CTA() {
  return (
    <section className="py-32 px-6 border-b border-[#27272a] bg-black text-center relative overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(234,88,12,0.06)_0%,_transparent_60%)] pointer-events-none" />
      <div className="absolute inset-0 dot-grid opacity-20 pointer-events-none" />

      <ScrollReveal className="max-w-3xl mx-auto relative z-10">
        <h2 className="text-5xl md:text-7xl font-bold tracking-tighter text-white mb-8 leading-none">
          Start building with <br /> FORGED.
        </h2>
        <p className="text-[#a1a1aa] text-lg md:text-xl leading-relaxed mb-12">
          A purely local, cryptographically secure enclave handling all your SSH
          connections.
        </p>
        <div className="flex items-center justify-center gap-6">
          <GlitchButton href="/login" className="h-12 px-10">Create Account</GlitchButton>
          <GlitchButton href="/docs" variant="secondary" className="h-12 px-10">Read Docs</GlitchButton>
        </div>
      </ScrollReveal>
    </section>
  );
}

function Footer() {
  return (
    <footer className="py-16 px-6 bg-black border-t border-[#27272a]">
      <div className="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-6">
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
      <SecurityCTA />
      <CTA />
      <Footer />
    </div>
  );
}
