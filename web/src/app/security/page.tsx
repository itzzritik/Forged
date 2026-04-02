import Link from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Security - Forged",
  description: "How Forged protects your SSH keys. Zero-knowledge architecture, encryption details, and threat model.",
};

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-16">
      <h2 className="text-xl font-medium tracking-tight mb-6">{title}</h2>
      {children}
    </section>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="p-4 rounded-xl bg-surface border border-border">
      <div className="text-xs text-accent-dim mb-1">{label}</div>
      <div className="text-sm text-zinc-300">{value}</div>
    </div>
  );
}

function ThreatRow({ threat, mitigation }: { threat: string; mitigation: string }) {
  return (
    <tr className="border-t border-border">
      <td className="py-3 pr-6 text-sm font-medium align-top">{threat}</td>
      <td className="py-3 text-sm text-muted">{mitigation}</td>
    </tr>
  );
}

export default function SecurityPage() {
  return (
    <>
      <nav className="w-full border-b border-border">
        <div className="max-w-3xl mx-auto px-6 h-14 flex items-center">
          <Link href="/" className="flex items-center gap-3">
            <span className="text-sm font-medium tracking-tight" style={{ fontFamily: "var(--font-mono)" }}>
              forged
            </span>
          </Link>
        </div>
      </nav>

      <main className="max-w-3xl mx-auto px-6 py-16">
        <h1 className="text-3xl font-medium tracking-tight mb-4">Security Model</h1>
        <p className="text-muted mb-12 leading-relaxed">
          Forged is built on zero-knowledge architecture. Your master password and private keys
          never leave your machine. The sync server stores only opaque encrypted blobs it cannot read.
        </p>

        <Section title="Encryption">
          <div className="grid sm:grid-cols-2 gap-4 mb-6">
            <Card label="Key Derivation" value="Argon2id (64MB memory, 3 iterations, 4 threads)" />
            <Card label="Vault Encryption" value="XChaCha20-Poly1305 (256-bit key, 24-byte nonce)" />
            <Card label="Sync Encryption" value="HKDF-SHA256 derived sync key + XChaCha20-Poly1305" />
            <Card label="Nonce Strategy" value="Random 24-byte per write (no reuse risk)" />
          </div>
          <p className="text-sm text-muted leading-relaxed">
            Your master password is processed through Argon2id, a memory-hard key derivation function
            that resists GPU and ASIC attacks. The derived 256-bit key encrypts the vault using
            XChaCha20-Poly1305, the same AEAD cipher used by WireGuard and age. The 24-byte nonce
            is randomly generated on every write, eliminating nonce-reuse risk even across synced devices.
          </p>
        </Section>

        <Section title="Key Hierarchy">
          <div className="p-5 rounded-xl bg-surface border border-border text-sm leading-8" style={{ fontFamily: "var(--font-mono)" }}>
            <div className="text-zinc-400">Master Password</div>
            <div className="text-zinc-500 ml-4">|</div>
            <div className="ml-4">
              <span className="text-accent-dim">Argon2id</span>
              <span className="text-zinc-500"> (salt_A)</span>
            </div>
            <div className="text-zinc-500 ml-4">|</div>
            <div className="ml-4 text-zinc-300">Vault Key (256-bit)</div>
            <div className="text-zinc-500 ml-8">|-- Encrypts local vault file</div>
            <div className="text-zinc-500 ml-8">|</div>
            <div className="ml-8">
              <span className="text-accent-dim">HKDF-SHA256</span>
              <span className="text-zinc-500"> (context: &quot;forged-sync&quot;)</span>
            </div>
            <div className="text-zinc-500 ml-12">|</div>
            <div className="ml-12 text-zinc-300">Sync Key</div>
            <div className="text-zinc-500 ml-16">+-- Encrypts vault blob for cloud upload</div>
          </div>
          <p className="text-sm text-muted mt-4 leading-relaxed">
            The server authenticates you via OAuth (Google/GitHub) but has no access to the vault key.
            Authentication and encryption are completely separate concerns.
          </p>
        </Section>

        <Section title="What the Server Sees">
          <div className="space-y-3">
            {[
              { label: "Your email", value: "Yes, for account identity (via OAuth)" },
              { label: "Your encrypted vault", value: "Yes, as an opaque blob it cannot decrypt" },
              { label: "Your master password", value: "Never. It never leaves your machine." },
              { label: "Your vault encryption key", value: "Never. Derived locally from master password." },
              { label: "Your private SSH keys", value: "Never. Encrypted inside the vault blob." },
            ].map((item) => (
              <div key={item.label} className="flex items-start gap-4 p-4 rounded-xl bg-surface border border-border">
                <div className="text-sm font-medium w-48 shrink-0">{item.label}</div>
                <div className="text-sm text-muted">{item.value}</div>
              </div>
            ))}
          </div>
        </Section>

        <Section title="Threat Model">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr>
                  <th className="pb-3 text-left font-medium w-48">Threat</th>
                  <th className="pb-3 text-left font-medium text-muted">Mitigation</th>
                </tr>
              </thead>
              <tbody>
                <ThreatRow threat="Disk theft / lost laptop" mitigation="Vault encrypted with Argon2id + XChaCha20-Poly1305. Without master password, vault is opaque bytes." />
                <ThreatRow threat="Server compromise" mitigation="Zero-knowledge. Server stores only encrypted blobs. No plaintext keys ever leave the client." />
                <ThreatRow threat="Memory dump / swap" mitigation="Key memory pages locked with mlock(). Daemon zeroes key material on shutdown." />
                <ThreatRow threat="Agent socket snooping" mitigation="Socket file permissions set to 0600. Only the owning user can connect." />
                <ThreatRow threat="MITM on sync" mitigation="TLS for transport. Vault payload independently encrypted with client-side key. Double encryption." />
                <ThreatRow threat="Master password brute force" mitigation="Argon2id with high parameters (64MB memory, 3 iterations). Rate limiting on cloud login." />
                <ThreatRow threat="Rogue device" mitigation="New device registration requires approval from an existing device." />
                <ThreatRow threat="Vault corruption" mitigation="Atomic writes (tmp + fsync + rename). File locking prevents concurrent access." />
              </tbody>
            </table>
          </div>
        </Section>

        <Section title="Memory Safety">
          <p className="text-sm text-muted leading-relaxed mb-4">
            Private keys are held in memory pages locked with <code className="text-zinc-400">mlock()</code> to
            prevent swapping to disk. On shutdown, all key material is explicitly zeroed.
          </p>
          <p className="text-sm text-muted leading-relaxed">
            <strong className="text-zinc-300">Known limitation:</strong> Go&apos;s garbage collector may copy heap objects
            before they are zeroed. We mitigate with mlock and best-effort zeroing. For production-grade
            mitigation, memguard or mmap-based allocation outside the Go heap is planned for a future release.
          </p>
        </Section>

        <Section title="Open Source">
          <p className="text-sm text-muted leading-relaxed">
            Forged is source-available. Every line of code is auditable.
            The encryption implementation uses well-established Go standard library and{" "}
            <code className="text-zinc-400">golang.org/x/crypto</code> packages, not custom cryptography.
          </p>
          <div className="mt-4">
            <a
              href="https://github.com/itzzritik/forged"
              className="inline-flex items-center gap-2 text-sm text-accent hover:text-amber-400 transition-colors"
            >
              View source on GitHub
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M7 17l9.2-9.2M17 17V7H7" />
              </svg>
            </a>
          </div>
        </Section>
      </main>
    </>
  );
}
