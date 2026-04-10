import Link from "next/link";
import type { Metadata } from "next";
import { ScrollReveal, SpotlightCard, GlitchButton } from "@/components/client";

export const metadata: Metadata = {
  title: "Security - Forged",
  description: "How Forged protects your SSH keys. Zero-knowledge architecture, encryption details, and threat model.",
};

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-24">
      <div className="flex items-center gap-4 mb-8">
        <div className="h-px bg-[#ea580c]/30 flex-1" />
        <h2 className="text-2xl font-bold tracking-widest text-white uppercase">{title}</h2>
      </div>
      {children}
    </section>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <SpotlightCard className="p-6 bg-black border border-[#27272a] group hover:border-[#ea580c]/50 transition-all duration-300">
      <div className="h-px bg-[#ea580c] scale-x-0 group-hover:scale-x-100 transition-transform origin-left duration-500 ease-out absolute top-0 left-0 w-full" />
      <div className="text-xs text-[#27272a] uppercase tracking-widest font-mono mb-3">{label}</div>
      <div className="text-sm font-mono text-white group-hover:text-[#ea580c] transition-colors">{value}</div>
    </SpotlightCard>
  );
}

function ThreatRow({ threat, mitigation }: { threat: string; mitigation: string }) {
  return (
    <tr className="border-b border-[#27272a] bg-black hover:bg-[#09090b] transition-colors group">
      <td className="py-5 pr-6 text-sm font-mono align-top text-white border-r border-[#27272a] pl-6 group-hover:text-[#ea580c] transition-colors w-1/3">
        {threat}
      </td>
      <td className="py-5 text-sm text-[#a1a1aa] pl-6 pr-6 leading-relaxed">
        {mitigation}
      </td>
    </tr>
  );
}

export default function SecurityPage() {
  return (
    <div className="bg-black min-h-screen relative overflow-hidden">
      <div className="absolute inset-0 dot-grid opacity-20 pointer-events-none" />
      <div className="absolute right-0 top-0 bottom-0 w-px bg-[#27272a] hidden lg:block mr-8" />
      <div className="absolute left-0 top-0 bottom-0 w-px bg-[#27272a] hidden lg:block ml-8" />

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
            <Link href="/docs" className="text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
              Docs
            </Link>
            <a href="https://github.com/itzzritik/forged" className="text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase">
              GitHub
            </a>
          </div>
        </div>
      </nav>

      <main className="max-w-[1000px] mx-auto px-6 pt-32 pb-32 relative z-10">
        <ScrollReveal className="text-center mb-24">
          <div className="inline-flex items-center justify-center gap-2 px-3 py-1.5 bg-[#ea580c] text-black text-[11px] font-bold uppercase tracking-[0.15em] font-mono mb-6">
            <span className="w-1.5 h-1.5 rounded-full bg-black animate-pulse" />
            Whitepaper
          </div>
          <h1 className="text-5xl md:text-7xl font-bold tracking-tighter mb-8 text-white leading-none">
            Security Protocol.
          </h1>
          <p className="text-[#a1a1aa] text-lg md:text-xl leading-relaxed max-w-2xl mx-auto">
            Forged is built on pure zero-knowledge architecture. The master password never moves. Key material is heavily insulated. Nothing escapes encrypted buffers.
          </p>
        </ScrollReveal>

        <ScrollReveal>
          <Section title="Encryption Primitives">
            <div className="grid sm:grid-cols-2 gap-px bg-[#27272a] border border-[#27272a] mb-8">
              <Card label="Key Derivation" value="Argon2id (64MB memory, 3 iterations)" />
              <Card label="Vault Encryption" value="XChaCha20-Poly1305 (256-bit key)" />
              <Card label="Sync Protocol" value="HKDF-SHA256 derived sync key" />
              <Card label="Nonce Execution" value="Random 24-byte per write" />
            </div>
            <p className="text-[13px] text-[#a1a1aa] font-mono leading-relaxed bg-[#09090b] p-4 border border-[#27272a] border-l-4 border-l-[#ea580c]">
              Derived 256-bit keys encrypt the vault using XChaCha20-Poly1305. The 24-byte nonce is freshly randomized directly on every local vault sync action, entirely preventing collision attacks.
            </p>
          </Section>
        </ScrollReveal>

        <ScrollReveal>
          <Section title="Key Hierarchy">
            <div className="p-8 bg-black border border-[#27272a] text-[13px] leading-8 overflow-x-auto shadow-2xl relative font-mono">
              <div className="absolute top-0 right-0 p-4 opacity-10 pointer-events-none">
                <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /><polyline points="3.27 6.96 12 12.01 20.73 6.96" /><line x1="12" y1="22.08" x2="12" y2="12" /></svg>
              </div>
              <div className="text-white font-bold inline-block border-b border-white/20 pb-1">Master Password</div>
              <div className="text-[#27272a] ml-4">|</div>
              <div className="ml-4">
                <span className="text-[#ea580c] font-bold">Argon2id</span>
                <span className="text-[#a1a1aa]"> (salt_A)</span>
              </div>
              <div className="text-[#27272a] ml-4">|</div>
              <div className="ml-4 text-white font-bold inline-block border-b border-white/20 pb-1">Vault Key (256-bit)</div>
              <div className="text-[#27272a] ml-8">|-- Encrypts local vault file</div>
              <div className="text-[#27272a] ml-8">|</div>
              <div className="ml-8">
                <span className="text-[#ea580c] font-bold">HKDF-SHA256</span>
                <span className="text-[#a1a1aa]"> (context: &quot;forged-sync&quot;)</span>
              </div>
              <div className="text-[#27272a] ml-12">|</div>
              <div className="ml-12 text-white font-bold inline-block border-b border-white/20 pb-1">Sync Key</div>
              <div className="text-[#27272a] ml-16">+-- Encrypts vault blob upload</div>
            </div>
          </Section>
        </ScrollReveal>

        <ScrollReveal>
          <Section title="Payload Access Controls">
            <div className="space-y-px bg-[#27272a] border border-[#27272a]">
              {[
                { label: "Email Address", value: "Available strictly for account ID metadata (via OAuth)" },
                { label: "Raw Vault Blob", value: "Encrypted payload accessible to sync servers" },
                { label: "Master Password", value: "Blocked via hardware. Never leaves local execution." },
                { label: "Encryption Key", value: "Local-only deterministic generation. Invisible to servers." },
                { label: "Private SSH Keys", value: "Nested within AES wrapped buffers. Invisible." },
              ].map((item) => (
                <div key={item.label} className="relative flex flex-col sm:flex-row items-start sm:items-center gap-4 p-5 bg-black hover:bg-[#09090b] group transition-colors">
                  <div className="absolute left-0 top-0 bottom-0 w-1 bg-transparent group-hover:bg-[#ea580c] transition-colors" />
                  <div className="text-[13px] font-bold text-white font-mono uppercase tracking-widest w-full sm:w-64 shrink-0 px-2">{item.label}</div>
                  <div className="text-[13px] text-[#a1a1aa] font-mono tracking-wide px-2">{item.value}</div>
                </div>
              ))}
            </div>
          </Section>
        </ScrollReveal>

        <ScrollReveal>
          <Section title="Threat Model Matrix">
            <div className="border border-[#27272a] overflow-hidden shadow-2xl">
              <table className="w-full text-sm bg-black">
                <thead className="bg-[#09090b]">
                  <tr>
                    <th className="py-4 text-left font-bold text-[10px] tracking-widest text-[#27272a] uppercase border-r border-[#27272a] pl-6 w-1/3">Threat Vector</th>
                    <th className="py-4 text-left font-bold text-[10px] tracking-widest text-[#27272a] uppercase pl-6 w-2/3">Operational Mitigation</th>
                  </tr>
                </thead>
                <tbody>
                  <ThreatRow threat="Disk theft" mitigation="Vault heavily encrypted via Argon2id. Physical data remains opaque without brute forcing memory limits." />
                  <ThreatRow threat="Network Node capture" mitigation="Absolute Zero-knowledge constraints. Captured node clusters contain exclusively encrypted blobs." />
                  <ThreatRow threat="Resident swap" mitigation="Key memory pages rigorously locked with mlock(). Daemon actively zeroes memory block locations upon shutdown." />
                  <ThreatRow threat="Socket MITM" mitigation="Daemon socket bounds rigorously set to 0600 root-mapped profiles enforcing owner exclusivity." />
                  <ThreatRow threat="MITM on TLS Sync" mitigation="Forced TLS 1.3 protocol transit with secondary vault encryption. Double layer proxy protection." />
                  <ThreatRow threat="Brute force attack" mitigation="Impenetrable Argon2id parameters. Rate limiting strictly enforced server side." />
                  <ThreatRow threat="File corruption" mitigation="Write atomic logic implemented (tmp + fsync + rename) ensuring failure-free sync overwrites." />
                </tbody>
              </table>
            </div>
          </Section>
        </ScrollReveal>

        <ScrollReveal className="mt-24 text-center">
          <GlitchButton href="https://github.com/itzzritik/forged" external className="h-12 px-10">Examine Source Code</GlitchButton>
        </ScrollReveal>
      </main>
    </div>
  );
}
