"use client";

import * as DialogPrimitive from "@radix-ui/react-dialog";
import { cn } from "@/lib/utils";

interface ModalProps {
  title: string;
  closable?: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
  className?: string;
}

export const Modal = ({
  title,
  closable = true,
  open,
  onOpenChange,
  children,
  className,
}: ModalProps) => {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={closable ? onOpenChange : undefined}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <DialogPrimitive.Content
          className={cn(
            "fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2",
            "bg-card border border-border overflow-hidden shadow-2xl",
            "font-mono w-full max-w-md",
            "data-[state=open]:animate-in data-[state=closed]:animate-out",
            "data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
            "data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
            className,
          )}
          onEscapeKeyDown={closable ? undefined : (e) => e.preventDefault()}
          onPointerDownOutside={closable ? undefined : (e) => e.preventDefault()}
          onInteractOutside={closable ? undefined : (e) => e.preventDefault()}
        >
          {/* Terminal title bar */}
          <div className="flex items-center h-[38px] px-3.5 bg-[#0a0a0a] border-b border-border">
            <div className="flex items-center gap-[7px]">
              <button
                type="button"
                onClick={() => closable && onOpenChange(false)}
                className={cn(
                  "w-[11px] h-[11px] rounded-full border transition-colors",
                  closable
                    ? "bg-[#ef4444] border-[#dc2626] cursor-pointer hover:brightness-110"
                    : "bg-[#2a1215] border-[#3f1c20] cursor-default",
                )}
                disabled={!closable}
                aria-label="Close"
              />
              <div
                className="w-[11px] h-[11px] rounded-full bg-[#2a2510] border border-[#3f3615]"
                aria-hidden
              />
              <div
                className="w-[11px] h-[11px] rounded-full bg-[#0f2a18] border border-[#1a3f25]"
                aria-hidden
              />
            </div>
            <div className="flex-1 text-center text-[10px] tracking-[0.1em] text-muted-foreground uppercase">
              {title}
            </div>
            <div className="w-[51px]" />
          </div>
          {children}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
};
