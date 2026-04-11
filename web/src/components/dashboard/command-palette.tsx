"use client";

import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { toast } from "sonner";
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command";
import { VaultKeyMetadata } from "@/lib/vault-crypto";

interface CommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  keys: VaultKeyMetadata[];
}

const KeyIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="7.5" cy="15.5" r="5.5" />
    <path d="M21 2l-9.6 9.6" />
    <path d="M15.5 7.5l3 3L22 7l-3-3" />
  </svg>
);

const NavIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="9 18 15 12 9 6" />
  </svg>
);

const ThemeIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="5" />
    <line x1="12" y1="1" x2="12" y2="3" />
    <line x1="12" y1="21" x2="12" y2="23" />
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
    <line x1="1" y1="12" x2="3" y2="12" />
    <line x1="21" y1="12" x2="23" y2="12" />
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
  </svg>
);

const LockIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
    <path d="M7 11V7a5 5 0 0 1 10 0v4" />
  </svg>
);

const CopyIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
  </svg>
);

const navItems = [
  { label: "SSH Keys", href: "/dashboard", shortcut: "G K" },
  { label: "Devices", href: "/dashboard/devices", shortcut: "G D" },
  { label: "Account", href: "/dashboard/account", shortcut: "G A" },
];

export const CommandPalette = ({ open, onOpenChange, keys }: CommandPaletteProps) => {
  const router = useRouter();
  const { theme, setTheme } = useTheme();

  const run = (fn: () => void) => {
    onOpenChange(false);
    fn();
  };

  const copyPublicKey = async (key: VaultKeyMetadata) => {
    await navigator.clipboard.writeText(key.publicKey);
    toast.success(`Copied public key for "${key.name}"`);
  };

  const toggleTheme = () => {
    const next = theme === "dark" ? "light" : theme === "light" ? "system" : "dark";
    setTheme(next);
    toast.success(`Theme set to ${next}`);
  };

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} className="max-w-[calc(100vw-2rem)] sm:max-w-lg">
      <CommandInput placeholder="Search keys, actions, navigation..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>

        {keys.length > 0 && (
          <>
            <CommandGroup heading="SSH Keys">
              {keys.map((key) => {
                const hosts = key.hostRules.map((r) => r.match).join(", ");
                return (
                  <CommandItem
                    key={key.id}
                    value={`key-${key.name}-${key.type}`}
                    onSelect={() => run(() => copyPublicKey(key))}
                  >
                    <KeyIcon />
                    <span className="flex-1 truncate">{key.name}</span>
                    <span className="text-xs text-muted-foreground truncate max-w-50">
                      {key.type}{hosts ? ` · ${hosts}` : ""}
                    </span>
                    <CommandShortcut>Copy</CommandShortcut>
                  </CommandItem>
                );
              })}
            </CommandGroup>
            <CommandSeparator />
          </>
        )}

        <CommandGroup heading="Navigation">
          {navItems.map((item) => (
            <CommandItem
              key={item.href}
              value={`nav-${item.label}`}
              onSelect={() => run(() => router.push(item.href))}
            >
              <NavIcon />
              <span>{item.label}</span>
              <CommandShortcut>{item.shortcut}</CommandShortcut>
            </CommandItem>
          ))}
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Actions">
          <CommandItem
            value="action-toggle-theme"
            onSelect={() => run(toggleTheme)}
          >
            <ThemeIcon />
            <span>Toggle Theme</span>
            <CommandShortcut>T</CommandShortcut>
          </CommandItem>
          <CommandItem
            value="action-lock-vault"
            onSelect={() => run(() => {
              window.dispatchEvent(new CustomEvent("vault:lock"));
            })}
          >
            <LockIcon />
            <span>Lock Vault</span>
          </CommandItem>
          {keys.length > 0 && (
            <CommandItem
              value="action-copy-public-key"
              onSelect={() => run(() => copyPublicKey(keys[0]))}
            >
              <CopyIcon />
              <span>Copy Public Key</span>
              <CommandShortcut>C</CommandShortcut>
            </CommandItem>
          )}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
};
