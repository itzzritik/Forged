"use client";

import { toast } from "sonner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { VaultKeyMetadata } from "@/lib/vault-crypto";

interface KeyTableProps {
  keys: VaultKeyMetadata[];
}

export const KeyTable = ({ keys }: KeyTableProps) => {
  if (keys.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-2 text-center">
        <p className="text-sm text-muted-foreground">No keys in vault</p>
        <p className="text-xs font-mono text-muted-foreground">
          Add keys via CLI:{" "}
          <span className="text-primary">forged add &lt;name&gt;</span>
        </p>
      </div>
    );
  }

  const handleCopy = async (key: VaultKeyMetadata) => {
    try {
      await navigator.clipboard.writeText(key.publicKey);
      toast.success("Public key copied to clipboard");
    } catch {
      toast.error("Failed to copy to clipboard");
    }
  };

  const handleExport = (key: VaultKeyMetadata) => {
    toast.info(`Export via CLI: forged export ${key.name}`);
  };

  return (
    <TooltipProvider>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead className="hidden sm:table-cell">Fingerprint</TableHead>
            <TableHead>Hosts</TableHead>
            <TableHead className="hidden sm:table-cell">Signing</TableHead>
            <TableHead>Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {keys.map((key) => (
            <TableRow key={key.id}>
              <TableCell>
                <span className="font-medium text-foreground">{key.name}</span>
                <div className="text-xs text-muted-foreground">{key.type}</div>
              </TableCell>
              <TableCell className="hidden sm:table-cell">
                <Tooltip>
                  <TooltipTrigger render={<span />} className="font-mono text-sm text-muted-foreground block max-w-[180px] truncate cursor-default">
                    {key.fingerprint}
                  </TooltipTrigger>
                  <TooltipContent>
                    <span className="font-mono">{key.fingerprint}</span>
                  </TooltipContent>
                </Tooltip>
              </TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {key.hostRules.length > 0 ? (
                    key.hostRules.map((rule, i) => (
                      <Badge
                        key={i}
                        className="bg-primary/10 text-primary border border-primary/20 hover:bg-primary/10"
                      >
                        {rule.match}
                      </Badge>
                    ))
                  ) : (
                    <span className="text-xs text-muted-foreground">--</span>
                  )}
                </div>
              </TableCell>
              <TableCell className="hidden sm:table-cell">
                {key.gitSigning ? (
                  <span className="flex items-center gap-1.5 text-sm text-green-500">
                    <span className="size-1.5 rounded-full bg-green-500 shrink-0" />
                    Active
                  </span>
                ) : (
                  <span className="text-sm text-muted-foreground">Off</span>
                )}
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopy(key)}
                  >
                    Copy
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleExport(key)}
                  >
                    Export
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TooltipProvider>
  );
};
