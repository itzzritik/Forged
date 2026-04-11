"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Modal } from "@/components/ui/modal";

interface AccountData {
  name: string;
  email: string;
  provider: string;
}

export const AccountCards = () => {
  const router = useRouter();
  const [account, setAccount] = useState<AccountData | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteInput, setDeleteInput] = useState("");
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    fetch("/api/vault/account")
      .then((r) => r.json())
      .then((data) => setAccount(data))
      .catch(() => {});
  }, []);

  const handleDelete = async () => {
    if (deleteInput !== "DELETE") return;
    setDeleting(true);
    try {
      await fetch("/api/vault/account-delete", { method: "POST" });
      router.push("/");
    } finally {
      setDeleting(false);
    }
  };

  const initial = account?.name?.[0]?.toUpperCase() ?? "?";

  return (
    <div className="flex flex-col gap-4 max-w-2xl">
      {/* Profile Card */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-4">
          <div className="flex size-16 shrink-0 items-center justify-center rounded-full bg-orange-500 text-2xl font-semibold text-white">
            {initial}
          </div>
          <div className="min-w-0">
            <p className="text-lg font-semibold truncate">
              {account?.name ?? "--"}
            </p>
            <p className="text-sm text-muted-foreground truncate">
              {account?.email ?? "--"}
            </p>
            {account?.provider && (
              <Badge variant="outline" className="mt-1.5 capitalize">
                {account.provider}
              </Badge>
            )}
          </div>
        </div>
      </div>

      {/* Security Card */}
      <div className="rounded-lg border border-border bg-card p-6">
        <h2 className="text-sm font-semibold mb-4">Master Password</h2>
        <div className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Change your master password via CLI:
          </p>
          <code className="inline-block rounded bg-muted px-3 py-1.5 font-mono text-sm text-foreground w-fit">
            forged change-password
          </code>
          <Separator />
          <p className="text-sm text-muted-foreground">
            Vault Timeout: 4 hours of inactivity
          </p>
        </div>
      </div>

      {/* Danger Zone Card */}
      <div className="rounded-lg border border-destructive/50 bg-card p-6">
        <h2 className="text-sm font-semibold text-destructive mb-1">
          Delete Account
        </h2>
        <p className="text-sm text-muted-foreground mb-4">
          Permanently delete your account and all data
        </p>
        <Button
          variant="destructive"
          size="sm"
          onClick={() => setDeleteOpen(true)}
        >
          Delete Account
        </Button>
      </div>

      <Modal
        title="Account // Delete"
        closable={true}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
      >
        <div className="p-5 flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">
            This action is irreversible. Type{" "}
            <span className="font-mono text-foreground">DELETE</span> to
            confirm.
          </p>
          <Input
            placeholder="DELETE"
            value={deleteInput}
            onChange={(e) => setDeleteInput(e.target.value)}
            autoFocus
          />
          <Button
            variant="destructive"
            disabled={deleteInput !== "DELETE" || deleting}
            onClick={handleDelete}
          >
            {deleting ? "Deleting..." : "Delete Account"}
          </Button>
        </div>
      </Modal>
    </div>
  );
};
