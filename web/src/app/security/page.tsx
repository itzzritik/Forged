import Link from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Security - Forged",
  description: "How Forged protects your SSH keys. Zero-knowledge architecture, encryption details, and threat model.",
};

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-20">
      <div className="text-[10px] text-zinc-600 uppercase tracking-widest font-mono mb-2">Details</div>
      <h2 className="text-2xl font-bold tracking-tight mb-8 text-white">{title}</h2>
      {children}
    </section>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="p-6 rounded border border-white/10 bg-black hover:border-white/20 transition-colors">
      <div className="text-xs text-zinc-500 uppercase tracking-widest font-mono mb-3">{label}</div>
      <div className="text-sm font-mono text-white/90">{value}</div>
    </div>
  );
}

function ThreatRow({ threat, mitigation }: { threat: string; mitigation: string }) {
  return (
    <tr className="border-t border-white/10 bg-black hover:bg-white/5 transition-colors">
      <td className="py-4 pr-6 text-sm font-semibold align-top text-white border-r border-white/10 pl-4">{threat}</td>
      <td className="py-4 text-sm text-zinc-400 pl-6 pr-4">{mitigation}</td>
    </tr>
  );
}

export default function SecurityPage() {
  return (
    <div className="bg-black min-h-screen">
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-white/10 bg-black/50 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-6 h-14 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3 hover:opacity-80 transition-opacity">
             <span
              className="text-sm font-semibold tracking-widest text-white uppercase"
              style={{ fontFamily: "var(--font-mono)" }}
             >
                forged
             </span>
          </Link>
          <Link
            href="/docs"
            className="text-xs tracking-wider uppercase text-zinc-400 hover:text-white transition-colors"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            Docs
          </Link>
        </div>
      </nav>

      <main className="max-w-4xl mx-auto px-6 py-32">
        <h1 className="text-5xl md:text-6xl font-bold tracking-tighter mb-6 text-white">Security Protocol</h1>
        <p className="text-zinc-400 text-lg md:text-xl mb-16 leading-relaxed max-w-2xl">
          Forged is built on pure zero-knowledge architecture. The master password never moves. Key material is heavily insulated. Nothing escapes encrypted buffers.
        </p>

        <Section title="Encryption">
          <div className="grid sm:grid-cols-2 gap-4 mb-8">
            <Card label="Key Derivation" value="Argon2id (64MB memory, 3 iterations)" />
            <Card label="Vault Encryption" value="XChaCha20-Poly1305 (256-bit key)" />
            <Card label="Sync Protocol" value="HKDF-SHA256 derived sync key" />
            <Card label="Nonce Execution" value="Random 24-byte per write" />
          </div>
          <p className="text-sm text-zinc-400 leading-relaxed max-w-3xl">
            Derived 256-bit keys encrypt the vault using XChaCha20-Poly1305. The 24-byte nonce is freshly randomized directly on every local vault sync action, entirely preventing collision attacks.
          </p>
        </Section>

        <Section title="Key Hierarchy">
          <div className="p-8 rounded bg-black border border-white/20 text-xs sm:text-sm leading-8 overflow-x-auto shadow-2xl" style={{ fontFamily: "var(--font-mono)" }}>
            <div className="text-white font-bold">Master Password</div>
            <div className="text-white/20 ml-4">|</div>
            <div className="ml-4">
              <span className="text-zinc-500 font-bold">Argon2id</span>
              <span className="text-white/30"> (salt_A)</span>
            </div>
            <div className="text-white/20 ml-4">|</div>
            <div className="ml-4 text-white font-bold">Vault Key (256-bit)</div>
            <div className="text-white/20 ml-8">|-- Encrypts local vault file</div>
            <div className="text-white/20 ml-8">|</div>
            <div className="ml-8">
              <span className="text-zinc-500 font-bold">HKDF-SHA256</span>
              <span className="text-white/30"> (context: &quot;forged-sync&quot;)</span>
            </div>
            <div className="text-white/20 ml-12">|</div>
            <div className="ml-12 text-white font-bold">Sync Key</div>
            <div className="text-white/20 ml-16">+-- Encrypts vault blob upload</div>
          </div>
        </Section>

        <Section title="Payload Access">
          <div className="space-y-4">
            {[
              { label: "Email Address", value: "Available strictly for account ID metadata (via OAuth)" },
              { label: "Raw Vault Blob", value: "Encrypted payload accessible to sync servers" },
              { label: "Master Password", value: "Blocked via hardware. Never leaves local execution." },
              { label: "Encryption Key", value: "Local-only deterministic generation. Invisible to servers." },
              { label: "Private SSH Keys", value: "Nested within AES wrapped buffers. Invisible." },
            ].map((item) => (
              <div key={item.label} className="flex flex-col sm:flex-row items-start gap-4 p-5 rounded border border-white/10 bg-black">
                <div className="text-sm font-bold text-white uppercase tracking-wider w-full sm:w-64 shrink-0">{item.label}</div>
                <div className="text-sm text-zinc-400 font-mono tracking-wide">{item.value}</div>
              </div>
            ))}
          </div>
        </Section>

        <Section title="Threat Model matrix">
          <div className="rounded border border-white/20 overflow-hidden shadow-2xl">
            <table className="w-full text-sm">
              <thead className="bg-white/5">
                <tr>
                  <th className="py-4 text-left font-bold text-[10px] tracking-widest text-zinc-500 uppercase border-r border-white/10 pl-4 w-1/3">Threat Vector</th>
                  <th className="py-4 text-left font-bold text-[10px] tracking-widest text-zinc-500 uppercase pl-6 w-2/3">Operational Mitigation</th>
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
        
        <Section title="Source Code">
           <a
              href="https://github.com/itzzritik/forged"
              className="inline-flex h-12 px-6 bg-white text-black rounded items-center justify-center text-sm font-bold tracking-wide uppercase hover:bg-zinc-200 transition-colors"
            >
              Examine Source
            </a>
        </Section>

      </main>
    </div>
  );
}
