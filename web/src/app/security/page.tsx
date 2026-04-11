import Link from "next/link";
import type { Metadata } from "next";
import { ScrollReveal, GlitchButton } from "@/components/client";

export const metadata: Metadata = {
  title: "Security - Forged",
  description: "How Forged protects your SSH keys. Zero-knowledge architecture, encryption details, and threat model.",
};

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
          <Link href="/security" className="hidden sm:block text-[12px] tracking-wider text-white transition-colors uppercase">
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

function HeroBackground() {
  // Pre-generated hex lines for the background - deterministic, no runtime randomness
  const hexLines = [
    "e3b0c44298fc1c14 9afbf4c8996fb924 27ae41e4649b934c a495991b7852b855",
    "a7ffc6f8bf1ed766 51c14756a061d662 f580ff4de43b49fa 82d80a4b80f8434a",
    "6b86b273ff34fce1 9d6b804eff5a3f57 47ada4eaa22f1d49 c01e52ddb7875b4b",
    "d4735e3a265e16ee a03239cc719ab018 06b6fa084a4e97eb 22f9fe5e40b7e3e3",
    "4e07408562bedb8b 60ce05c1decfe3ad 16b72230967de01f 640b7e4729b49fce",
    "4b227777d4dd1fc6 1c6f884f48641d02 b4d121d3fd328cb0 8b5f0c25b5e33c7b",
    "ef2d127de37b942b aad06145e54b0c61 9a1f22327b2ebbcf bec78f5564afe39d",
    "e7f6c011776e8db7 cd330b54174fd76f 7d0216b612387a5f faf4e6c34b67ddfe",
    "2c624232cdd221771 294750a4d5e4d4cd 2bf3d5e56b3327e3 19c45e0ad9f1d2b8",
    "19581e27de7ced00 ff1ce50b2047e7a5 67c43b8d4b1ae21e 4db1a0c5c6e47e28",
    "4a44dc15364204a8 0fe80e9039455cc1 608281820fe2b24f 1e5233ade6af1dd5",
    "9f14025af0065b30 6c0a68d2efb68d65 de27cc87fd346d6e a25a9e1c2e6e1e7b",
    "b17ef6d19c7a5b1e e83b907af917c8b5 00c1a073db9c2aad eb44ec546aa3b90b",
    "ca978112ca1bbdca fac231b39a23dc4d a786eff8147c4e72 b9807785afee48bb",
    "3e23e8160039594a 33894f6564e1b134 8bbd7a0088d42c4a cb73eeaed59c009d",
    "2e7d2c03a9507ae2 65ecf5b5356885a5 394c32af5064a65a 2b6577725ec7d8a1",
  ];

  return (
    <div className="absolute inset-0 pointer-events-none select-none overflow-hidden">
      {/* Hex digest lines - scrolling upward */}
      <div className="absolute inset-0 flex flex-col justify-center gap-[2px] opacity-[0.06]">
        {hexLines.map((line, i) => (
          <div
            key={i}
            className="whitespace-nowrap font-mono text-[11px] lg:text-[13px] tracking-[0.3em] text-white animate-slide-up"
            style={{
              animationDelay: `${i * 120}ms`,
              animationDuration: '1.2s',
              animationFillMode: 'both',
            }}
          >
            {line}
          </div>
        ))}
      </div>

      {/* Gradient overlays for depth */}
      <div className="absolute inset-0 bg-gradient-to-b from-black via-transparent to-black" />
      <div className="absolute inset-0 bg-gradient-to-r from-black via-black/60 to-transparent" />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_bottom_left,_rgba(234,88,12,0.1)_0%,_transparent_50%)]" />
    </div>
  );
}

function Hero() {
  const stats = [
    { label: "Cipher", value: "XChaCha20" },
    { label: "KDF", value: "Argon2id" },
    { label: "Key", value: "256-bit" },
    { label: "Nonce", value: "192-bit" },
  ];

  return (
    <section className="relative h-dvh flex flex-col border-b border-[#27272a] overflow-hidden">
      <HeroBackground />

      {/* Content */}
      <div className="relative z-10 flex-1 flex flex-col justify-center px-6 lg:px-16">
        <div className="max-w-5xl">
          <div className="animate-slide-up">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 bg-[#ea580c] text-black text-[11px] font-bold uppercase tracking-[0.15em] font-mono">
              <span className="w-1.5 h-1.5 rounded-full bg-black animate-pulse" />
              Security Whitepaper
            </div>
          </div>

          <h1 className="text-[clamp(48px,10vw,128px)] font-bold tracking-[-0.04em] leading-[0.85] text-white mt-8 animate-slide-up delay-100">
            Security<br />Protocol
          </h1>

          <p className="text-lg md:text-xl text-[#a1a1aa] max-w-2xl mt-8 leading-relaxed animate-slide-up delay-200">
            Zero-knowledge architecture. The master password never moves. Key material is heavily insulated. Nothing escapes encrypted buffers.
          </p>

          {/* Cryptographic stat badges */}
          <div className="flex flex-wrap items-center gap-3 mt-10 animate-slide-up delay-300">
            {stats.map((s) => (
              <div key={s.label} className="flex items-center gap-3 h-10 border border-[#27272a] bg-[#09090b] px-4 hover:border-[#ea580c]/40 transition-colors group">
                <span className="text-[9px] font-mono tracking-widest uppercase text-[#a1a1aa]">{s.label}</span>
                <span className="w-px h-4 bg-[#27272a]" />
                <span className="text-[12px] font-mono tracking-wider text-white font-bold group-hover:text-[#ea580c] transition-colors">{s.value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Bottom bar */}
      <div className="relative z-10 border-t border-[#27272a] bg-[#050505]/80 backdrop-blur-sm shrink-0">
        <div className="px-6 lg:px-16 h-14 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <div className="flex items-center gap-2">
              <span className="w-1.5 h-1.5 bg-[#10b981] animate-pulse rounded-full shadow-[0_0_8px_#10b981]" />
              <span className="text-[10px] text-[#10b981] font-mono tracking-widest uppercase font-bold">Audited</span>
            </div>
            <span className="w-px h-4 bg-[#27272a] hidden sm:block" />
            <span className="text-[10px] text-[#a1a1aa] font-mono tracking-widest uppercase hidden sm:block">MIT Licensed</span>
            <span className="w-px h-4 bg-[#27272a] hidden md:block" />
            <span className="text-[10px] text-[#a1a1aa] font-mono tracking-widest uppercase hidden md:block">Open Source</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-[9px] font-mono tracking-widest text-[#3f3f46] uppercase hidden sm:block">golang.org/x/crypto</span>
          </div>
        </div>
      </div>
    </section>
  );
}

function EncryptionPrimitives() {
  const primitives = [
    {
      icon: "M12 15v2m-6 4h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2zm10-10V7a4 4 0 0 0-8 0v4h8z",
      title: "Argon2id",
      subtitle: "Key derivation",
      desc: "Winner of the Password Hashing Competition. Memory-hard function making GPU and ASIC brute-force attacks economically infeasible.",
      bullets: ["64MB memory cost", "3 iterations, 4 parallelism", "ASIC-resistant by design"],
      cta: "View Parameters",
      href: "#key-hierarchy",
    },
    {
      icon: "M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0 1 12 2.944a11.955 11.955 0 0 1-8.618 3.04A12.02 12.02 0 0 0 3 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z",
      title: "XChaCha20-Poly1305",
      subtitle: "Vault encryption",
      desc: "Extended-nonce AEAD stream cipher. Immune to timing attacks, nonce-misuse resistant, and faster than AES-GCM without hardware acceleration.",
      bullets: ["256-bit encryption key", "192-bit extended nonce", "Authenticated encryption (AEAD)"],
      cta: "View Cipher Details",
      href: "#key-hierarchy",
    },
    {
      icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
      title: "HKDF-SHA256",
      subtitle: "Sync protocol",
      desc: "Deterministic key derivation using HMAC-based Extract-and-Expand. Sync keys are mathematically isolated from vault keys.",
      bullets: ["Context-bound derivation", "Isolated sync key space", "Forward secrecy"],
      cta: "View Sync Protocol",
      href: "/docs#sync",
    },
    {
      icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 0 0 4.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 0 1-15.357-2m15.357 2H15",
      title: "Random 24-byte",
      subtitle: "Nonce strategy",
      desc: "Every vault write generates a cryptographically random 24-byte nonce, entirely preventing collision attacks across billions of operations.",
      bullets: ["Per-write randomization", "No nonce reuse possible", "Collision-resistant by design"],
      cta: "View Nonce Strategy",
      href: "/docs",
    },
  ];

  return (
    <section className="relative py-24 lg:py-36 px-6 lg:px-16 bg-black border-b border-[#27272a] overflow-hidden">
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <div className="relative z-10 w-full">
        <ScrollReveal className="mb-4">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Encryption Primitives</span>
          </div>
        </ScrollReveal>

        <ScrollReveal>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl xl:text-8xl font-bold tracking-tighter leading-[0.95] text-white text-pretty">
            Cryptographic Foundation
          </h2>
        </ScrollReveal>

        <ScrollReveal className="mt-6">
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed">
            Every layer of the vault is built on battle-tested, open-standard cryptographic primitives. No proprietary algorithms, no security through obscurity.
          </p>
        </ScrollReveal>

        <div className="relative z-10 mt-16 border-t border-l border-white/10 grid grid-cols-1 md:grid-cols-2">
          {primitives.map((f) => (
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

        <ScrollReveal delay={0.2}>
          <div className="mt-px bg-[#09090b] border border-[#27272a] border-t-0 p-6 lg:p-8">
            <div className="flex items-start gap-4">
              <div className="w-1 h-full min-h-[40px] bg-[#ea580c] shrink-0 self-stretch" />
              <p className="text-[13px] text-[#a1a1aa] font-mono leading-relaxed">
                Derived 256-bit keys encrypt the vault using XChaCha20-Poly1305. The 24-byte nonce is freshly randomized on every local vault sync action, entirely preventing collision attacks. All cryptographic operations use Go&apos;s <code className="text-white">golang.org/x/crypto</code> library - no custom implementations.
              </p>
            </div>
          </div>
        </ScrollReveal>
      </div>
    </section>
  );
}

function KeyHierarchy() {
  const steps = [
    { id: "01", label: "Input", value: "Master Password", desc: "User-provided passphrase. Never stored on disk, never transmitted. Zeroed from memory immediately after derivation." },
    { id: "02", label: "KDF", value: "Argon2id", desc: "Memory-hard function transforms the password into a 256-bit key. 64MB memory, 3 iterations, 4 parallelism. ASIC-resistant." },
    { id: "03", label: "Output", value: "Vault Key", desc: "256-bit symmetric key encrypts and decrypts the local vault file via XChaCha20-Poly1305 AEAD. Never persisted." },
    { id: "04", label: "Expand", value: "HKDF-SHA256", desc: "Derives a mathematically isolated sync key from the vault key using context-bound HMAC Extract-and-Expand." },
    { id: "05", label: "Output", value: "Sync Key", desc: "Encrypts the vault blob before upload. The sync server only ever sees opaque, encrypted binary data." },
  ];

  return (
    <section id="key-hierarchy" className="relative py-24 lg:py-36 bg-black border-b border-[#27272a] overflow-hidden">
      <div className="absolute inset-0 opacity-[0.02]" style={{ backgroundImage: "linear-gradient(#fff 1px, transparent 1px), linear-gradient(90deg, #fff 1px, transparent 1px)", backgroundSize: "64px 64px" }} />

      <div className="relative z-10 w-full px-6 lg:px-16">
        <ScrollReveal className="mb-4">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Key Hierarchy</span>
          </div>
        </ScrollReveal>

        <ScrollReveal>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl xl:text-8xl font-bold tracking-tighter leading-[0.95] text-white text-pretty">
            Derivation Chain
          </h2>
        </ScrollReveal>

        <ScrollReveal className="mt-6 mb-16">
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed">
            A single master password deterministically generates all encryption keys through a strict, one-way derivation chain. No key is ever stored - they are regenerated on demand.
          </p>
        </ScrollReveal>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-px bg-[#27272a]">
          {steps.map((step, i) => (
            <ScrollReveal key={step.id} delay={i * 0.1} className="flex flex-col bg-black p-8 group relative overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-br from-[#ea580c]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />

              <div className="flex items-center justify-between mb-8">
                <span className="text-[#a1a1aa] font-mono text-xs tracking-widest uppercase">Step // {step.id}</span>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="text-[#3f3f46] group-hover:text-[#ea580c] transition-colors duration-500">
                  <path d="M5 12h14" />
                  <path d="M12 5l7 7-7 7" />
                </svg>
              </div>
              <span className="text-sm font-mono tracking-widest uppercase text-[#ea580c] mb-1 opacity-80">{step.label}</span>
              <span className="text-2xl sm:text-3xl text-white font-bold tracking-tight mb-6">{step.value}</span>
              <p className="text-[#a1a1aa] text-sm leading-relaxed mt-auto relative z-10">
                {step.desc}
              </p>
            </ScrollReveal>
          ))}
        </div>
      </div>
    </section>
  );
}

function PayloadAccess() {
  const groups = [
    {
      status: "Visible",
      color: "#a1a1aa",
      items: [
        { label: "Email Address", desc: "Available strictly for account ID metadata (via OAuth)" },
      ],
    },
    {
      status: "Encrypted",
      color: "#f59e0b",
      items: [
        { label: "Raw Vault Blob", desc: "Encrypted payload accessible to sync servers - opaque binary data" },
      ],
    },
    {
      status: "Blocked",
      color: "#ef4444",
      items: [
        { label: "Master Password", desc: "Never leaves local execution. Zeroed from memory after derivation." },
        { label: "Encryption Key", desc: "Local-only deterministic generation. Invisible to any server." },
        { label: "Private SSH Keys", desc: "Nested within AEAD-encrypted vault buffers. Physically inaccessible." },
      ],
    },
  ];

  return (
    <section className="relative py-24 lg:py-36 bg-black border-b border-[#27272a] overflow-hidden">
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <div className="relative z-10 w-full px-6 lg:px-16">
        <ScrollReveal className="mb-4">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Access Control</span>
          </div>
        </ScrollReveal>

        <ScrollReveal>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl xl:text-8xl font-bold tracking-tighter leading-[0.95] text-white text-pretty">
            Payload Visibility
          </h2>
        </ScrollReveal>

        <ScrollReveal className="mt-6 mb-16">
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed">
            What the server can see, what it can&apos;t, and what is physically impossible to access - even with full infrastructure compromise.
          </p>
        </ScrollReveal>

        <div className="flex flex-col gap-4">
          {groups.map((group, gi) => (
            <ScrollReveal key={group.status} delay={gi * 0.1}>
              <div className="border border-[#27272a] bg-black overflow-hidden flex flex-col md:flex-row">
                {/* Left status strip */}
                <div className="md:w-48 shrink-0 border-b md:border-b-0 md:border-r border-[#27272a] bg-[#050505] p-6 flex md:flex-col items-center md:items-start justify-between md:justify-center gap-3">
                  <div className="flex items-center gap-2.5">
                    <span className="w-2 h-2 rounded-full animate-pulse" style={{ backgroundColor: group.color, boxShadow: `0 0 8px ${group.color}` }} />
                    <span className="text-sm font-mono font-bold tracking-widest uppercase" style={{ color: group.color }}>
                      {group.status}
                    </span>
                  </div>
                  <span className="text-[10px] font-mono text-[#3f3f46] tracking-widest">{group.items.length} {group.items.length === 1 ? "field" : "fields"}</span>
                </div>

                {/* Items */}
                <div className="flex-1 divide-y divide-[#18181b]">
                  {group.items.map((item) => (
                    <div key={item.label} className="group flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-8 p-6 hover:bg-[#09090b] transition-colors">
                      <span className="text-[13px] font-mono font-bold text-white sm:w-48 shrink-0 group-hover:text-[#ea580c] transition-colors duration-300">
                        {item.label}
                      </span>
                      <p className="text-[13px] text-[#a1a1aa] leading-relaxed">{item.desc}</p>
                    </div>
                  ))}
                </div>
              </div>
            </ScrollReveal>
          ))}
        </div>
      </div>
    </section>
  );
}

function ThreatModel() {
  const threats = [
    { vector: "Disk theft", mitigation: "Vault is encrypted via Argon2id + XChaCha20-Poly1305. Physical data is cryptographically opaque without brute-forcing memory-hard key derivation." },
    { vector: "Network node capture", mitigation: "Zero-knowledge architecture. Captured infrastructure contains exclusively encrypted binary blobs with no decryption capability." },
    { vector: "Memory swap leak", mitigation: "Key memory pages locked with unix.Mlock(). Daemon actively zeroes all sensitive memory regions upon shutdown or lock." },
    { vector: "Socket interception", mitigation: "Daemon socket permissions strictly enforced at 0600 with owner-only access. No remote socket exposure." },
    { vector: "MITM on TLS sync", mitigation: "Forced TLS 1.3 transit with secondary vault-level encryption. Dual-layer protection makes MITM cryptographically useless." },
    { vector: "Brute force attack", mitigation: "Argon2id parameters (64MB, 3 iterations) make each guess cost ~300ms. Rate limiting strictly enforced server-side." },
    { vector: "File corruption", mitigation: "Atomic write logic (tmp + fsync + rename) ensures vault integrity. Write failures never corrupt existing vault state." },
  ];

  return (
    <section className="relative py-24 lg:py-36 bg-black border-b border-[#27272a] overflow-hidden">
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <div className="relative z-10 w-full px-6 lg:px-16">
        <ScrollReveal className="mb-16">
          <div className="flex items-center gap-2.5 mb-4">
            <span className="h-2 w-2 bg-[#ea580c]" />
            <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Threat Model</span>
          </div>
          <h2 className="text-4xl sm:text-5xl lg:text-7xl font-bold tracking-tighter leading-[0.95] text-white mb-6">
            Attack Surface
          </h2>
          <p className="text-base lg:text-lg text-[#a1a1aa] max-w-2xl leading-relaxed">
            Every known attack vector, mapped to its operational mitigation. If you find a gap, we want to hear about it.
          </p>
        </ScrollReveal>

        {/* Desktop: Table */}
        <ScrollReveal className="hidden md:block">
          <div className="border border-[#27272a] overflow-hidden">
            <table className="w-full text-sm bg-black">
              <thead>
                <tr className="bg-[#050505]">
                  <th className="py-5 text-left font-mono font-bold text-[10px] tracking-widest text-[#ea580c] uppercase border-r border-[#27272a] pl-8 w-[280px]">Threat Vector</th>
                  <th className="py-5 text-left font-mono font-bold text-[10px] tracking-widest text-[#ea580c] uppercase pl-8">Operational Mitigation</th>
                </tr>
              </thead>
              <tbody>
                {threats.map((t) => (
                  <tr key={t.vector} className="border-t border-[#18181b] bg-black hover:bg-[#09090b] transition-colors group">
                    <td className="py-6 pr-6 text-sm font-mono align-top text-white border-r border-[#27272a] pl-8 group-hover:text-[#ea580c] transition-colors duration-300">
                      {t.vector}
                    </td>
                    <td className="py-6 text-sm text-[#a1a1aa] pl-8 pr-8 leading-relaxed">
                      {t.mitigation}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </ScrollReveal>

        {/* Mobile: Stacked cards */}
        <div className="md:hidden space-y-px">
          {threats.map((t, i) => (
            <ScrollReveal key={t.vector} delay={i * 0.05}>
              <div className="bg-black border border-[#27272a] p-6 group hover:border-[#ea580c]/30 transition-colors">
                <span className="text-sm font-mono text-white font-bold group-hover:text-[#ea580c] transition-colors block mb-3">{t.vector}</span>
                <p className="text-sm text-[#a1a1aa] leading-relaxed">{t.mitigation}</p>
              </div>
            </ScrollReveal>
          ))}
        </div>
      </div>
    </section>
  );
}

function SecurityCTA() {
  return (
    <section className="relative py-36 bg-black overflow-hidden text-center flex flex-col items-center justify-center">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(234,88,12,0.06)_0%,_transparent_60%)]" />
      <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }} />

      <ScrollReveal className="relative z-10 w-full px-6 lg:px-16 max-w-3xl flex flex-col items-center">
        <div className="flex items-center gap-2.5 mb-6 justify-center">
          <span className="h-2 w-2 bg-[#ea580c] animate-pulse" />
          <span className="text-[10px] font-mono tracking-[0.2em] text-[#a1a1aa] uppercase">Open Source</span>
        </div>
        <h2 className="text-4xl sm:text-5xl lg:text-7xl font-bold tracking-tighter leading-[0.95] text-white text-pretty mb-8">
          Don&apos;t trust us.<br />Audit us.
        </h2>
        <p className="text-base lg:text-xl text-[#a1a1aa] leading-relaxed mb-12 max-w-2xl">
          The entire Forged core is open source under MIT. Every cryptographic implementation, every daemon operation, every vault interaction - fully inspectable.
        </p>
        <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
          <GlitchButton href="https://github.com/itzzritik/forged" external className="h-14 px-10 text-sm">Examine Source Code</GlitchButton>
          <GlitchButton href="/docs" variant="secondary" className="h-14 px-10 text-sm">Read Docs</GlitchButton>
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
          <Link href="/" className="hover:text-[#ea580c] transition-colors">
            Home
          </Link>
        </div>
      </div>
    </footer>
  );
}

export default function SecurityPage() {
  return (
    <div className="bg-black min-h-screen">
      <Nav />
      <Hero />
      <EncryptionPrimitives />
      <KeyHierarchy />
      <PayloadAccess />
      <ThreatModel />
      <SecurityCTA />
      <Footer />
    </div>
  );
}
